package cli

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	encodingjson "encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
)

const (
	EventIntakeEndpoint     = "https://intake-analytics.wikimedia.org/v1/events?hasty=true"
	TelemetrySkipAnnotation = "a-wikimedia-cli/skip-telemetry"
	// Under normal operation, if we are writing to our expected logging file
	normalEventSendInterval = time.Hour
	// Under fallback operation, if we are writing to a temporary logging file (that would otherwise vanish on system reboot / cleanup)
	fallbackEventSendInterval = time.Minute
	eventLockStaleAge         = 2 * time.Minute
)

type Events struct {
	EventFile    string
	FallbackDir  string
	usedFallback atomic.Bool
}

// EventFilePath returns the normal event queue path for the current user.
// This is .events in the user's home directory.
// However the fallback now is suffixed with .jsonl, and maybe this should migrate to .jsonl at some point too...
func EventFilePath() string {
	return filepath.Join(UserDirectoryPath(), ".events")
}

func NewEvents(eventFile string) *Events {
	identity, err := filepath.Abs(eventFile)
	if err != nil {
		identity = filepath.Clean(eventFile)
	}
	pathHash := sha256.Sum256([]byte(filepath.Clean(identity)))
	return &Events{
		EventFile:   eventFile,
		FallbackDir: filepath.Join(os.TempDir(), AgentName, "telemetry", fmt.Sprintf("%x", pathHash[:12])),
	}
}

func (e *Events) FallbackEventFile() string {
	return filepath.Join(e.FallbackDir, "events.jsonl")
}

func (e *Events) EventPaths() []string {
	return []string{e.EventFile, e.FallbackEventFile()}
}

func (e *Events) currentDtString() string {
	return time.Now().Format("2006-01-02T15:04:05.000Z")
}

// AddCommandRunEvent derives the canonical command name, never its arguments.
func (e *Events) AddCommandRunEvent(cmd *cobra.Command, version Version) {
	e.addCommandEvent(cmd, version, false)
}

func (e *Events) AddCommandHelpEvent(cmd *cobra.Command, version Version) {
	e.addCommandEvent(cmd, version, true)
}

func (e *Events) addCommandEvent(cmd *cobra.Command, version Version, help bool) {
	cmd = cobrautil.ExecutingCommand(cmd)
	command := cobrautil.FullCommandStringWithoutPrefix(cmd, cmd.Root().Name())
	if help {
		command = strings.TrimSpace(command + " --help")
	} else if command == "" {
		// A root command execution must have a name too.
		command = cmd.Root().Name()
	}
	if strings.TrimSpace(command) == "" {
		logrus.Debug("Not recording an unnamed command")
		return
	}
	e.addEventToStore(map[string]interface{}{
		"$schema": "/analytics/mwcli/command_execute/1.0.0",
		"meta": map[string]interface{}{
			"stream": "mwcli.command_execute",
		},
		"dt":      e.currentDtString(),
		"command": command,
		"version": version,
	})
}

func (e *Events) AddFeatureUsageEvent(feature string, version Version) {
	e.addEventToStore(map[string]interface{}{
		"$schema": "/analytics/mwcli/feature_usage/1.0.0",
		"meta": map[string]interface{}{
			"stream": "mwcli.feature_usage",
		},
		"dt":      e.currentDtString(),
		"feature": feature,
		"version": version,
	})
}

func (e *Events) addEventToStore(event map[string]interface{}) {
	if !Opts.Telemetry || os.Getenv("MWCLI_CONTEXT_TEST") != "" {
		return
	}
	encoded, err := encodingjson.Marshal(event)
	if err != nil {
		logrus.Warnf("Could not encode telemetry event: %v", err)
		return
	}
	if err := e.appendEvent(string(encoded)); err != nil {
		logrus.Warnf("Could not queue telemetry event; continuing without recording it: %v", err)
	}
}

func (e *Events) appendEvent(event string) error {
	unlock, err := acquireEventsLock(filepath.Join(e.FallbackDir, "queue.lock"), time.Second)
	if err != nil {
		if !errors.Is(err, errEventsLockTimeout) {
			if primaryErr := appendEventLine(e.EventFile, event); primaryErr == nil {
				logrus.Warnf("Could not lock the temporary telemetry queue; saved the event to %s without queue locking", e.EventFile)
				return nil
			}
		}
		return fmt.Errorf("could not lock telemetry queue: %w", err)
	}
	defer unlock()

	if err := appendEventLine(e.EventFile, event); err == nil {
		return nil
	} else {
		primaryErr := err
		if fallbackErr := appendEventLine(e.FallbackEventFile(), event); fallbackErr != nil {
			return fmt.Errorf("primary event file %s: %v; fallback event file %s: %w", e.EventFile, primaryErr, e.FallbackEventFile(), fallbackErr)
		}
		e.usedFallback.Store(true)
		logrus.Warnf("Could not write telemetry to %s; queued it in the temporary fallback file %s", e.EventFile, e.FallbackEventFile())
		return nil
	}
}

func appendEventLine(path, line string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Clean(path), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	n, err := file.WriteString(line + "\n")
	if err != nil {
		return err
	}
	if n != len(line)+1 {
		return io.ErrShortWrite
	}
	return file.Sync()
}

// RawEvents returns queued events from both the normal and temporary files,
// including batches retained after a failed send.
func (e *Events) RawEvents() []string {
	unlock, err := acquireEventsLock(filepath.Join(e.FallbackDir, "queue.lock"), time.Second)
	if err != nil {
		logrus.Warnf("Could not lock telemetry queue for reading: %v", err)
		return e.readPendingEvents()
	}
	defer unlock()
	return e.readPendingEvents()
}

func (e *Events) readPendingEvents() []string {
	paths := e.pendingBatchPaths()
	paths = append(paths, e.EventPaths()...)
	var events []string
	for _, path := range paths {
		lines, err := readEventLines(path)
		if err != nil {
			logrus.Warnf("Could not read pending telemetry from %s: %v", path, err)
			continue
		}
		events = append(events, lines...)
	}
	return events
}

// MaybeEmitEvents sends queued events if the timing limit permits it.
func (e *Events) MaybeEmitEvents() bool {
	return e.emitEvents(false)
}

// EmitEvents sends queued events without waiting for the timing limit.
func (e *Events) EmitEvents() bool {
	return e.emitEvents(true)
}

// Queue files are atomically rotated before sending so concurrent commands can
// continue recording events while the HTTP request is in flight.
func (e *Events) emitEvents(force bool) bool {
	if !Opts.Telemetry || os.Getenv("MWCLI_CONTEXT_TEST") != "" {
		return false
	}
	if err := os.MkdirAll(e.FallbackDir, 0o700); err != nil {
		logrus.Warnf("Could not create temporary telemetry directory %s: %v", e.FallbackDir, err)
		return false
	}

	unlockSend, err := acquireEventsLock(filepath.Join(e.FallbackDir, "send.lock"), 100*time.Millisecond)
	if err != nil {
		// Another process is already sending, or has just sent. Leave the queue
		// untouched; a later command will pick it up.
		return false
	}
	defer unlockSend()

	unlockQueue, err := acquireEventsLock(filepath.Join(e.FallbackDir, "queue.lock"), time.Second)
	if err != nil {
		logrus.Warnf("Could not lock telemetry queue for sending: %v", err)
		return false
	}
	queueLocked := true
	defer func() {
		if queueLocked {
			unlockQueue()
		}
	}()

	now := time.Now()
	if !force {
		lastAttempt, err := e.lastAttemptTime()
		if err != nil {
			logrus.Warnf("Could not read telemetry send limit: %v", err)
			return false
		}
		interval := normalEventSendInterval
		if e.usedFallback.Load() {
			interval = fallbackEventSendInterval
		}
		if !lastAttempt.IsZero() && now.Sub(lastAttempt) < interval {
			return false
		}
	}

	for _, queuePath := range e.EventPaths() {
		if err := e.rotateQueue(queuePath, now); err != nil {
			// A read-only primary queue must not prevent sending events from the
			// fallback queue. Its contents remain in place for a later writable run.
			logrus.Warnf("Could not rotate telemetry queue %s: %v", queuePath, err)
		}
	}
	var batchPaths []string
	for i, path := range e.pendingBatchPaths() {
		queuePath := e.FallbackEventFile()
		if strings.HasPrefix(path, e.EventFile+".pending-") {
			queuePath = e.EventFile
		}
		claimedPath := fmt.Sprintf("%s.pending-%d-%d-%d", queuePath, now.UnixNano(), os.Getpid(), i)
		if err := os.Rename(path, claimedPath); err != nil {
			// A read-only batch cannot be removed after sending. Leave it for
			// a writable run instead of repeatedly sending the same events.
			logrus.Warnf("Could not claim telemetry batch %s: %v", path, err)
			continue
		}
		batchPaths = append(batchPaths, claimedPath)
	}
	var eventJSON []string
	for _, path := range batchPaths {
		lines, err := readEventLines(path)
		if err != nil {
			logrus.Warnf("Could not read pending telemetry batch %s: %v", path, err)
			return false
		}
		eventJSON = append(eventJSON, lines...)
	}
	if len(eventJSON) == 0 {
		return false
	}
	if err := e.writeLastAttempt(now); err != nil {
		logrus.Warnf("Could not record telemetry send limit: %v", err)
		return false
	}

	// Queue mutations are serialized, but do not hold the queue lock over the
	// network request. The send lock still prevents simultaneous uploads.
	unlockQueue()
	queueLocked = false

	payload := []byte("[" + strings.Join(eventJSON, ",") + "]")
	req, err := http.NewRequest(http.MethodPost, EventIntakeEndpoint, bytes.NewReader(payload))
	if err != nil {
		logrus.Warnf("Could not create telemetry request: %v", err)
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", HTTPUserAgent)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logrus.Debugf("Telemetry submission failed: %v", err)
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		logrus.Debugf("Event submission failed with status code %d", resp.StatusCode)
		_, _ = io.Copy(io.Discard, resp.Body)
		return false
	}

	for _, path := range batchPaths {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			logrus.Warnf("Telemetry was sent, but pending batch %s could not be removed: %v", path, err)
		}
	}
	// Preserve the old behavior where the normal event file exists but is
	// empty after a successful submission when its directory is writable.
	if file, err := os.OpenFile(e.EventFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600); err == nil {
		_ = file.Close()
	}
	logrus.Debug("Event submission success")
	return true
}

func (e *Events) rotateQueue(path string, now time.Time) error {
	lines, err := readEventLines(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(lines) == 0 {
		return nil
	}
	batchPath := fmt.Sprintf("%s.pending-%d-%d", path, now.UnixNano(), os.Getpid())
	if _, err := os.Stat(batchPath); err == nil {
		batchPath = fmt.Sprintf("%s-%d", batchPath, time.Now().UnixNano())
	}
	return os.Rename(path, batchPath)
}

func (e *Events) pendingBatchPaths() []string {
	paths := make([]string, 0)
	for _, queuePath := range e.EventPaths() {
		dir := filepath.Dir(queuePath)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		prefix := filepath.Base(queuePath) + ".pending-"
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) {
				paths = append(paths, filepath.Join(dir, entry.Name()))
			}
		}
	}
	return paths
}

func (e *Events) lastAttemptTime() (time.Time, error) {
	contents, err := os.ReadFile(filepath.Join(e.FallbackDir, "last-attempt"))
	if errors.Is(err, os.ErrNotExist) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	lastAttempt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(string(contents)))
	if err != nil {
		logrus.Warnf("Ignoring invalid telemetry send timestamp: %v", err)
		return time.Time{}, nil
	}
	return lastAttempt, nil
}

func (e *Events) writeLastAttempt(attempt time.Time) error {
	path := filepath.Join(e.FallbackDir, "last-attempt")
	tmp, err := os.CreateTemp(e.FallbackDir, "last-attempt-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.WriteString(attempt.Format(time.RFC3339Nano)); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	// Remove first for platforms where rename does not replace an existing file.
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(tmpPath, path)
}

func readEventLines(path string) ([]string, error) {
	file, err := os.Open(filepath.Clean(path))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" {
			lines = append(lines, line)
		}
	}
	return lines, scanner.Err()
}
