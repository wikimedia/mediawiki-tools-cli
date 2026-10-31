package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPendingEventsInPathWithGlobCharacters(t *testing.T) {
	previousOptions, previousTransport := Opts, http.DefaultTransport
	t.Cleanup(func() { Opts, http.DefaultTransport = previousOptions, previousTransport })
	t.Setenv("MWCLI_CONTEXT_TEST", "")
	Opts.Telemetry = true
	dir := t.TempDir()
	e := NewEvents(filepath.Join(dir, "[config]", ".events"))
	e.FallbackDir = filepath.Join(dir, "fallback")
	e.AddFeatureUsageEvent("example", "test")
	var sent []json.RawMessage
	http.DefaultTransport = eventTransport(func(r *http.Request) (*http.Response, error) {
		if err := json.NewDecoder(r.Body).Decode(&sent); err != nil {
			return nil, err
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})
	if !e.EmitEvents() || len(sent) != 1 || len(e.RawEvents()) != 0 {
		t.Fatalf("expected one sent event and an empty queue; sent %d, queued %d", len(sent), len(e.RawEvents()))
	}
}

func TestReadOnlyPendingBatchWaitsForWritableRun(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires Unix directory permissions")
	}
	previousOptions, previousTransport := Opts, http.DefaultTransport
	t.Cleanup(func() { Opts, http.DefaultTransport = previousOptions, previousTransport })
	t.Setenv("MWCLI_CONTEXT_TEST", "")
	Opts.Telemetry = true
	dir := t.TempDir()
	primaryDir := filepath.Join(dir, "primary")
	if err := os.Mkdir(primaryDir, 0o700); err != nil {
		t.Fatal(err)
	}
	e := NewEvents(filepath.Join(primaryDir, ".events"))
	e.FallbackDir = filepath.Join(dir, "fallback")
	primaryBatch := e.EventFile + ".pending-old"
	if err := os.WriteFile(primaryBatch, []byte(`{"feature":"primary"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := appendEventLine(e.FallbackEventFile(), `{"feature":"fallback"}`); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(primaryDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(primaryDir, 0o700) })
	var sent []struct {
		Feature string `json:"feature"`
	}
	http.DefaultTransport = eventTransport(func(r *http.Request) (*http.Response, error) {
		if err := json.NewDecoder(r.Body).Decode(&sent); err != nil {
			t.Fatal(err)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})
	if !e.EmitEvents() || len(sent) != 1 || sent[0].Feature != "fallback" {
		t.Fatalf("sent = %+v, want fallback only", sent)
	}
	if _, err := os.Stat(primaryBatch); err != nil {
		t.Fatalf("primary batch was not retained: %v", err)
	}
	if err := os.Chmod(primaryDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if !e.EmitEvents() || len(sent) != 1 || sent[0].Feature != "primary" {
		t.Fatalf("sent = %+v, want retained primary", sent)
	}
}

func TestFallbackRetryAndRateLimit(t *testing.T) {
	previousOptions, previousTransport := Opts, http.DefaultTransport
	t.Cleanup(func() { Opts, http.DefaultTransport = previousOptions, previousTransport })
	t.Setenv("MWCLI_CONTEXT_TEST", "")
	Opts.Telemetry = true
	dir := t.TempDir()
	blocked := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	e := NewEvents(filepath.Join(blocked, ".events"))
	e.FallbackDir = filepath.Join(dir, "fallback")
	e.AddFeatureUsageEvent("fallback", "test")
	if len(e.RawEvents()) != 1 {
		t.Fatal("event was not saved to fallback")
	}

	calls := 0
	var sent []json.RawMessage
	http.DefaultTransport = eventTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if err := json.NewDecoder(r.Body).Decode(&sent); err != nil {
			t.Fatal(err)
		}
		status := http.StatusServiceUnavailable
		if calls > 1 {
			status = http.StatusOK
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})
	if e.MaybeEmitEvents() || calls != 1 || len(sent) != 1 {
		t.Fatal("failed send did not retain the fallback event")
	}
	if len(e.RawEvents()) != 1 {
		t.Fatal("failed send lost the event")
	}
	if e.MaybeEmitEvents() || calls != 1 {
		t.Fatal("send was not limited to once per minute")
	}
	if err := os.Remove(blocked); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(blocked, 0o700); err != nil {
		t.Fatal(err)
	}
	normal := NewEvents(e.EventFile)
	normal.FallbackDir = e.FallbackDir
	normal.AddFeatureUsageEvent("normal", "test")
	if err := os.WriteFile(filepath.Join(e.FallbackDir, "last-attempt"), []byte(time.Now().Add(-2*time.Minute).Format(time.RFC3339Nano)), 0o600); err != nil {
		t.Fatal(err)
	}
	if normal.MaybeEmitEvents() || calls != 1 {
		t.Fatal("normal operation sent again within an hour")
	}
	if err := os.WriteFile(filepath.Join(e.FallbackDir, "last-attempt"), []byte(time.Now().Add(-2*time.Hour).Format(time.RFC3339Nano)), 0o600); err != nil {
		t.Fatal(err)
	}
	if !normal.MaybeEmitEvents() || calls != 2 || len(sent) != 2 {
		t.Fatal("normal run did not send both queues")
	}
	if len(e.RawEvents()) != 0 {
		t.Fatal("successful send left queued events")
	}
}

func TestFallbackUsesOneMinuteInterval(t *testing.T) {
	previousOptions, previousTransport := Opts, http.DefaultTransport
	t.Cleanup(func() { Opts, http.DefaultTransport = previousOptions, previousTransport })
	t.Setenv("MWCLI_CONTEXT_TEST", "")
	Opts.Telemetry = true
	dir := t.TempDir()
	blocked := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	e := NewEvents(filepath.Join(blocked, ".events"))
	e.FallbackDir = filepath.Join(dir, "fallback")
	e.AddFeatureUsageEvent("fallback", "test")
	if err := os.WriteFile(filepath.Join(e.FallbackDir, "last-attempt"), []byte(time.Now().Add(-2*time.Minute).Format(time.RFC3339Nano)), 0o600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	http.DefaultTransport = eventTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})
	if !e.MaybeEmitEvents() || calls != 1 {
		t.Fatal("fallback did not send after one minute")
	}
}

func TestExplicitSubmitBypassesTimingLimit(t *testing.T) {
	previousOptions, previousTransport := Opts, http.DefaultTransport
	t.Cleanup(func() { Opts, http.DefaultTransport = previousOptions, previousTransport })
	t.Setenv("MWCLI_CONTEXT_TEST", "")
	Opts.Telemetry = true
	dir := t.TempDir()
	e := NewEvents(filepath.Join(dir, ".events"))
	e.FallbackDir = filepath.Join(dir, "fallback")
	calls := 0
	http.DefaultTransport = eventTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})
	e.AddFeatureUsageEvent("first", "test")
	if !e.MaybeEmitEvents() || calls != 1 {
		t.Fatal("initial send failed")
	}
	e.AddFeatureUsageEvent("second", "test")
	if e.MaybeEmitEvents() || calls != 1 {
		t.Fatal("automatic send ignored the timing limit")
	}
	if !e.EmitEvents() || calls != 2 {
		t.Fatal("explicit submit did not bypass the timing limit")
	}
}

// TestTelemetryProcessHelper runs in child processes started by the integration test.
func TestTelemetryProcessHelper(t *testing.T) {
	if os.Getenv("MWCLI_EVENT_PROCESS_HELPER") != "1" {
		t.Skip("only used by the integration test")
	}
	Opts.Telemetry = true
	e := NewEvents(os.Getenv("MWCLI_TEST_EVENT_FILE"))
	e.FallbackDir = os.Getenv("MWCLI_TEST_FALLBACK_DIR")
	http.DefaultTransport = eventTransport(func(r *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		f, err := os.OpenFile(os.Getenv("MWCLI_TEST_SENT_FILE"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		if _, err := f.Write(append(body, '\n')); err != nil {
			return nil, err
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})
	e.AddFeatureUsageEvent(fmt.Sprintf("process-%d", os.Getpid()), "test")
	e.MaybeEmitEvents()
}

func TestTelemetryAcrossProcesses(t *testing.T) {
	dir := t.TempDir()
	e := NewEvents(filepath.Join(dir, ".events"))
	e.FallbackDir = filepath.Join(dir, "fallback")
	sentFile := filepath.Join(dir, "sent.jsonl")
	const processes = 12
	commands := make([]*exec.Cmd, 0, processes)
	for i := 0; i < processes; i++ {
		cmd := exec.Command(os.Args[0], "-test.run=^TestTelemetryProcessHelper$")
		cmd.Env = append(os.Environ(), "MWCLI_CONTEXT_TEST=", "MWCLI_EVENT_PROCESS_HELPER=1", "MWCLI_TEST_EVENT_FILE="+e.EventFile, "MWCLI_TEST_FALLBACK_DIR="+e.FallbackDir, "MWCLI_TEST_SENT_FILE="+sentFile)
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		commands = append(commands, cmd)
	}
	for _, cmd := range commands {
		if err := cmd.Wait(); err != nil {
			t.Fatalf("child process: %v", err)
		}
	}
	f, err := os.Open(sentFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	requests, sent := 0, 0
	for scanner.Scan() {
		requests++
		var events []json.RawMessage
		if err := json.Unmarshal(scanner.Bytes(), &events); err != nil {
			t.Fatal(err)
		}
		sent += len(events)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("got %d requests, want one per minute", requests)
	}
	if total := sent + len(e.RawEvents()); total != processes {
		t.Fatalf("sent plus queued = %d, want %d", total, processes)
	}
}

func TestConcurrentTelemetryQueue(t *testing.T) {
	previousOptions, previousTransport := Opts, http.DefaultTransport
	t.Cleanup(func() { Opts, http.DefaultTransport = previousOptions, previousTransport })
	t.Setenv("MWCLI_CONTEXT_TEST", "")
	Opts.Telemetry = true
	dir := t.TempDir()
	e := NewEvents(filepath.Join(dir, ".events"))
	e.FallbackDir = filepath.Join(dir, "fallback")
	var mu sync.Mutex
	var sent int
	http.DefaultTransport = eventTransport(func(r *http.Request) (*http.Response, error) {
		var events []json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&events); err != nil {
			t.Error(err)
		}
		mu.Lock()
		sent += len(events)
		mu.Unlock()
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); e.AddFeatureUsageEvent("concurrent", "test"); e.MaybeEmitEvents() }()
	}
	wg.Wait()
	mu.Lock()
	count := sent
	mu.Unlock()
	if total := count + len(e.RawEvents()); total != 20 {
		t.Fatalf("sent plus queued = %d, want 20", total)
	}
}

func TestTelemetryQueueLockTimeoutPreservesEvent(t *testing.T) {
	previousOptions, previousTransport := Opts, http.DefaultTransport
	t.Cleanup(func() { Opts, http.DefaultTransport = previousOptions, previousTransport })
	t.Setenv("MWCLI_CONTEXT_TEST", "")
	Opts.Telemetry = true
	e := NewEvents(filepath.Join(t.TempDir(), ".events"))
	e.FallbackDir = filepath.Join(filepath.Dir(e.EventFile), "fallback")
	unlock, err := acquireEventsLock(filepath.Join(e.FallbackDir, "queue.lock"), 0)
	if err != nil {
		t.Fatal(err)
	}
	// Hold the lock past the writer's deadline, as a slow concurrent process
	// can do. The event must survive without appending to a locked queue.
	func() {
		defer unlock()
		e.AddFeatureUsageEvent("lock-timeout", "test")
	}()
	if events := e.RawEvents(); len(events) != 1 {
		t.Fatalf("queued %d events after lock timeout, want 1", len(events))
	}
	var sent []json.RawMessage
	http.DefaultTransport = eventTransport(func(r *http.Request) (*http.Response, error) {
		if err := json.NewDecoder(r.Body).Decode(&sent); err != nil {
			return nil, err
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})
	if !e.EmitEvents() || len(sent) != 1 || len(e.RawEvents()) != 0 {
		t.Fatalf("expected one sent event and an empty queue; sent %d, queued %d", len(sent), len(e.RawEvents()))
	}
}
