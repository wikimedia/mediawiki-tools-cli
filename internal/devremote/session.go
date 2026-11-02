package devremote

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	osexec "os/exec"
	ossignal "os/signal"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gofrs/flock"
)

// ValidateInvocation is supplied by the application so delegation uses the real
// Cobra definitions and argument validation, without executing command hooks.
var ValidateInvocation func([]string) error

// StartSession starts or reuses a detached controller, then opens the requested UI.
func StartSession(ctx context.Context, c Context, stdout, stderr io.Writer) error {
	if err := c.Options.validate(); err != nil {
		return err
	}
	if !safeName.MatchString(c.SourceName) {
		return errors.New("source-name must contain only letters, digits, underscores and hyphens")
	}
	if c.Editor == "code" {
		if err := checkEditor(ctx, stderr); err != nil {
			return fmt.Errorf("desktop editor unavailable; use --editor web or --editor none: %w", err)
		}
	}
	directory, _ := contextDirectory(c.Context)
	if err := os.MkdirAll(filepath.Dir(directory), 0o700); err != nil {
		return err
	}
	startLock := flock.New(startLockPath(c.Context))
	locked, err := startLock.TryLockContext(ctx, 100*time.Millisecond)
	if err != nil || !locked {
		return fmt.Errorf("could not lock session startup: %w", err)
	}
	defer startLock.Unlock()
	if err := SaveContext(c); err != nil {
		return err
	}
	if current, err := loadSession(c.Context); err == nil && current.Executable != "" && current.Executable != executableSignature() {
		_ = Disconnect(ctx, c.Context)
		for i := 0; i < 100 && controlRequest(ctx, c.Context, "/status", nil) == nil; i++ {
			time.Sleep(100 * time.Millisecond)
		}
	}
	logPath := filepath.Join(directory, "session.log")
	logOffset := int64(0)
	if info, err := os.Stat(logPath); err == nil {
		logOffset = info.Size()
	}
	if err := controlRequest(ctx, c.Context, "/status", nil); err != nil {
		if previous, err := loadSession(c.Context); err == nil && previous.SSHControl != "" {
			cleanup, stop := context.WithTimeout(ctx, 5*time.Second)
			(transport{args: c.SSHArgs}).command(cleanup, []string{"-S", previous.SSHControl, "-O", "exit"}, "").Run()
			stop()
		}
		if err := saveSession(c.Context, sessionState{Started: time.Now().UTC()}); err != nil {
			return err
		}
		dir, _ := contextDirectory(c.Context)
		log, err := os.OpenFile(filepath.Join(dir, "session.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		executable, err := os.Executable()
		if err != nil {
			log.Close()
			return err
		}
		process := osexec.Command(executable, "__remote-session", c.Context)
		process.Stdout, process.Stderr = log, log
		detach(process)
		err = process.Start()
		log.Close()
		if err != nil {
			return err
		}
		process.Process.Release()
		fmt.Fprintf(stdout, "Starting background session %s. Log: %s\n", c.Context, filepath.Join(dir, "session.log"))
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(30 * time.Minute)
	defer deadline.Stop()
	for {
		if log, err := os.Open(logPath); err == nil {
			log.Seek(logOffset, io.SeekStart)
			n, _ := io.Copy(stdout, io.LimitReader(log, 64<<10))
			logOffset += n
			log.Close()
		}
		s, err := loadSession(c.Context)
		if err == nil && s.Stopped {
			return errors.New("remote session was stopped during setup")
		}
		if err == nil && s.Error != "" {
			return fmt.Errorf("remote session: %s", s.Error)
		}
		if err == nil && s.Ready && controlRequest(ctx, c.Context, "/status", nil) == nil {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return errors.New("remote setup is still running; use remote status and inspect session.log")
		case <-ticker.C:
		}
	}
	c, err = LoadContext(c.Context)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Context: %s\nSource: %s\n", c.Context, c.Source)
	if len(c.Forward) > 0 {
		fmt.Fprintf(stdout, "Delegated to this machine for this session: %s\n", strings.Join(c.Forward, ", "))
	}
	if c.Directory != "" {
		fmt.Fprintf(stdout, "Local source: %s (continuous one-way sync; overwritten remote files are backed up)\n", c.Directory)
	}
	if c.GitRemote != "" {
		fmt.Fprintf(stdout, "Git destination (authenticated locally): %s\n", c.GitRemote)
	}
	return openSessionUI(ctx, c, stdout, stderr)
}

// RunSession owns listeners, synchronization and SSH forwards until disconnected.
func RunSession(parent context.Context, name string) (result error) {
	c, err := LoadContext(name)
	if err != nil {
		return err
	}
	dir, _ := contextDirectory(name)
	lock := flock.New(filepath.Join(dir, "session.lock"))
	locked, err := lock.TryLock()
	if err != nil || !locked {
		return err
	}
	defer lock.Unlock()
	ctx, cancel := ossignal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer cancel()
	temporary, err := os.MkdirTemp("", "mw-session-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	state := sessionState{PID: os.Getpid(), Control: controlAddress(temporary), SSHControl: sshControlPath(temporary), Executable: executableSignature(), BinaryRelease: sessionRelease(), Started: time.Now().UTC()}
	var stateMu, syncMu sync.Mutex
	update := func(ready bool, message string) {
		stateMu.Lock()
		defer stateMu.Unlock()
		state.Ready, state.Error = ready, message
		if err := saveSession(name, state); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
	defer func() {
		message := ""
		if result != nil {
			message = result.Error()
		}
		stateMu.Lock()
		state.Stopped = true
		stateMu.Unlock()
		update(false, message)
	}()
	control, err := listenControl(state.Control)
	if err != nil {
		return err
	}
	defer control.Close()
	bridge, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer bridge.Close()
	var randomKey [32]byte
	if _, err := rand.Read(randomKey[:]); err != nil {
		return err
	}
	bridgeKey := fmt.Sprintf("%x", randomKey[:])
	synchronize := func() error {
		syncMu.Lock()
		defer syncMu.Unlock()
		current, err := LoadContext(name)
		if err != nil {
			return err
		}
		if current.Directory == "" || current.Source == "" {
			return nil
		}
		return syncDirectory(ctx, current)
	}
	controlMux := http.NewServeMux()
	controlMux.HandleFunc("/status", func(w http.ResponseWriter, _ *http.Request) {
		stateMu.Lock()
		defer stateMu.Unlock()
		json.NewEncoder(w).Encode(state)
	})
	controlMux.HandleFunc("/stop", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		if flush, ok := w.(http.Flusher); ok {
			flush.Flush()
		}
		cancel()
	})
	controlMux.HandleFunc("/sync", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		stateMu.Lock()
		ready := state.Ready
		stateMu.Unlock()
		if !ready {
			http.Error(w, "session is connecting", http.StatusServiceUnavailable)
			return
		}
		if err := synchronize(); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
		}
	})
	controlServer := &http.Server{Handler: controlMux, ReadHeaderTimeout: 10 * time.Second}
	bridgeServer := &http.Server{Handler: authenticatedBridge(bridgeKey, bridgeHandler(name)), ReadHeaderTimeout: 10 * time.Second}
	go controlServer.Serve(control)
	go bridgeServer.Serve(bridge)
	defer controlServer.Close()
	defer bridgeServer.Close()
	update(false, "")
	hooks := connectionHooks{
		bridgePort: bridge.Addr().(*net.TCPAddr).Port,
		bridgeKey:  bridgeKey,
		sshControl: state.SSHControl,
		located: func(prepared preparedContext) error {
			c.Workspace, c.Metadata, c.Source, c.Port, c.WebPort = prepared.workspace, prepared.metadata, prepared.source, prepared.port, prepared.webPort
			return SaveContext(c)
		},
		prepare: func(prepared preparedContext) error {
			c.Workspace, c.Metadata, c.Source, c.Port, c.WebPort = prepared.workspace, prepared.metadata, prepared.source, prepared.port, prepared.webPort
			if err := SaveContext(c); err != nil {
				return err
			}
			if c.GitRemote != "" {
				if err := prepareGit(ctx, c); err != nil {
					return err
				}
			}
			if err := synchronize(); err != nil {
				return err
			}
			return nil
		},
		ready: func() {
			update(true, "")
		},
	}
	if c.Directory != "" {
		go func() {
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					stateMu.Lock()
					ready := state.Ready
					stateMu.Unlock()
					if !ready {
						continue
					}
					if err := synchronize(); err != nil && ctx.Err() == nil {
						fmt.Fprintln(os.Stderr, "Source synchronization:", err)
					}
				}
			}
		}()
	}
	// Retry broken SSH sessions. Initial preparation errors are reported instead
	// of repeatedly applying a failed recipe without user feedback.
	everReady := false
	for {
		stateMu.Lock()
		wasReady := state.Ready || everReady
		stateMu.Unlock()
		err := runConnection(ctx, c, hooks, os.Stdout, os.Stderr)
		if ctx.Err() != nil {
			return nil
		}
		stateMu.Lock()
		wasReady = wasReady || state.Ready
		everReady = wasReady
		stateMu.Unlock()
		if !wasReady {
			return err
		}
		update(false, "")
		fmt.Fprintln(os.Stderr, "Connection lost; reconnecting:", err)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(3 * time.Second):
		}
	}
}

func authenticatedBridge(key string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+key)) != 1 {
			http.Error(w, "invalid session capability", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Disconnect closes a background session without stopping its development services.
func Disconnect(ctx context.Context, name string) error {
	return controlRequest(ctx, name, "/stop", nil)
}

// FlushSync ensures local edits have arrived before a remote command runs.
func FlushSync(ctx context.Context, name string) error {
	return controlRequest(ctx, name, "/sync", nil)
}

// SessionStatus reports the active controller and source location.
func SessionStatus(ctx context.Context, name string) (string, error) {
	c, err := LoadContext(name)
	if err != nil {
		return "", err
	}
	status := "disconnected"
	if err := controlRequest(ctx, name, "/status", nil); err == nil {
		status = "connecting"
		if s, err := loadSession(name); err == nil && s.Ready {
			status = "connected"
		}
	}
	return fmt.Sprintf("%s: %s — %s — %s", name, status, c.SSHArgs[len(c.SSHArgs)-1], c.Source), nil
}

func controlRequest(ctx context.Context, name, endpoint string, body io.Reader) error {
	s, err := loadSession(name)
	if err != nil {
		return err
	}
	client := socketClient(s.Control)
	client.Timeout = 5 * time.Minute
	if endpoint == "/status" {
		client.Timeout = time.Second
	}
	defer client.CloseIdleConnections()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://session"+endpoint, body)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return errors.New(strings.TrimSpace(string(message)))
	}
	return nil
}

type frameWriter struct {
	mu      *sync.Mutex
	encoder *json.Encoder
	w       http.ResponseWriter
	stream  string
}

func (f frameWriter) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.encoder.Encode(frame{Stream: f.stream, Data: p}); err != nil {
		return 0, err
	}
	if flush, ok := f.w.(http.Flusher); ok {
		flush.Flush()
	}
	return len(p), nil
}

func bridgeHandler(name string) http.Handler {
	concurrent := make(chan struct{}, 4)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || (r.URL.Path != "/invoke" && r.URL.Path != "/git") {
			http.Error(w, "unsupported bridge operation", http.StatusForbidden)
			return
		}
		select {
		case concurrent <- struct{}{}:
			defer func() { <-concurrent }()
		case <-r.Context().Done():
			return
		}
		var request invocation
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 12<<20)).Decode(&request); err != nil {
			http.Error(w, "invalid or oversized invocation", http.StatusBadRequest)
			return
		}
		c, err := LoadContext(name)
		if err == nil && r.URL.Path == "/invoke" {
			err = validateForward(c, request.Args)
		}
		if err == nil && r.URL.Path == "/git" {
			err = validateGit(c, request)
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		encoder := json.NewEncoder(w)
		var mu sync.Mutex
		stdout := frameWriter{&mu, encoder, w, "stdout"}
		stderr := frameWriter{&mu, encoder, w, "stderr"}
		if r.URL.Path == "/git" {
			err = runGit(r.Context(), c, request, stdout, stderr)
		} else {
			executable, executableErr := os.Executable()
			if executableErr != nil {
				err = executableErr
			} else {
				args := append([]string{"--no-interaction"}, request.Args...)
				command := osexec.CommandContext(r.Context(), executable, args...)
				command.Env = append(os.Environ(), "MW_REMOTE_LOCAL=1")
				command.Stdin = strings.NewReader(string(request.Input))
				command.Stdout, command.Stderr = stdout, stderr
				err = command.Run()
			}
		}
		code := 0
		if err != nil {
			code = report(err, stderr)
		}
		encoder.Encode(frame{Exit: &code})
	})
}

func validateForward(c Context, args []string) error {
	if len(args) == 0 || !slices.Contains(c.Forward, "phab") || ValidateInvocation == nil {
		return errors.New("Phabricator delegation is not enabled; reopen locally with --forward phab")
	}
	return ValidateInvocation(args)
}
