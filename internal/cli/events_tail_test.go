package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func testEventTailer(t *testing.T) (*Events, *eventTailer) {
	t.Helper()
	dir := t.TempDir()
	events := NewEvents(filepath.Join(dir, ".events"))
	events.FallbackDir = filepath.Join(dir, "fallback")
	tailer := &eventTailer{events: events}
	t.Cleanup(tailer.close)
	return events, tailer
}

func expectTailedEvents(t *testing.T, tailer *eventTailer, want ...string) {
	t.Helper()
	got, err := tailer.read()
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("read = %q, %v; want %q", got, err, want)
	}
}

func TestEventTailerFollowsQueuesAndRotation(t *testing.T) {
	events, tailer := testEventTailer(t)
	expectTailedEvents(t, tailer)
	if err := appendEventLine(events.EventFile, `{"event":1}`); err != nil {
		t.Fatal(err)
	}
	if err := appendEventLine(events.FallbackEventFile(), `{"event":2}`); err != nil {
		t.Fatal(err)
	}
	expectTailedEvents(t, tailer, `{"event":1}`, `{"event":2}`)
	expectTailedEvents(t, tailer)

	// Both queues may have unread appends when they are rotated and claimed.
	for i, path := range events.EventPaths() {
		if err := appendEventLine(path, fmt.Sprintf(`{"rotated":%d}`, i)); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(path, path+".pending-claimed"); err != nil {
			t.Fatal(err)
		}
	}
	if err := appendEventLine(events.EventFile, `{"new":true}`); err != nil {
		t.Fatal(err)
	}
	expectTailedEvents(t, tailer, `{"rotated":0}`, `{"rotated":1}`, `{"new":true}`)
	expectTailedEvents(t, tailer)
	for _, path := range events.EventPaths() {
		if err := os.Remove(path + ".pending-claimed"); err != nil {
			t.Fatal(err)
		}
	}
	expectTailedEvents(t, tailer)
	if len(tailer.files) != 1 {
		t.Fatalf("retained %d files after upload", len(tailer.files))
	}
}

func TestEventTailerDrainsDeletedQueueAndFindsPendingBatches(t *testing.T) {
	events, tailer := testEventTailer(t)
	if err := appendEventLine(events.EventFile, `{"first":true}`); err != nil {
		t.Fatal(err)
	}
	expectTailedEvents(t, tailer, `{"first":true}`)
	if err := appendEventLine(events.EventFile, `{"before_upload":true}`); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(events.EventFile); err != nil {
		t.Fatal(err)
	}
	expectTailedEvents(t, tailer, `{"before_upload":true}`)
	if err := events.appendPendingEvent(`{"pending":true}`); err != nil {
		t.Fatal(err)
	}
	expectTailedEvents(t, tailer, `{"pending":true}`)
	paths := events.pendingBatchPaths()
	if len(paths) != 1 {
		t.Fatalf("pending batches = %v", paths)
	}
	if err := os.Rename(paths[0], events.FallbackEventFile()+".pending-renamed"); err != nil {
		t.Fatal(err)
	}
	expectTailedEvents(t, tailer)
}

func TestEventTailerPartialLinesAndTruncation(t *testing.T) {
	events, tailer := testEventTailer(t)
	if err := os.WriteFile(events.EventFile, []byte(`{"part":`), 0o600); err != nil {
		t.Fatal(err)
	}
	expectTailedEvents(t, tailer)
	if err := appendEventLine(events.EventFile, "true}\n\n"); err != nil {
		t.Fatal(err)
	}
	expectTailedEvents(t, tailer, `{"part":true}`)
	if err := os.WriteFile(events.EventFile, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	expectTailedEvents(t, tailer, "{}")
	expectTailedEvents(t, tailer)
}

func TestEventTailerInitialQueueOrder(t *testing.T) {
	events, tailer := testEventTailer(t)
	for _, path := range []string{events.EventFile, events.FallbackEventFile()} {
		if err := appendEventLine(path, `{"queued":true}`); err != nil {
			t.Fatal(err)
		}
		if err := appendEventLine(path+".pending-batch", `{"pending":true}`); err != nil {
			t.Fatal(err)
		}
	}
	want := events.RawEvents()
	got, err := tailer.read()
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("tail snapshot=%q err=%v, cat=%q", got, err, want)
	}
}

type tailedEventWriter struct {
	lines chan string
}

func (w tailedEventWriter) Write(data []byte) (int, error) {
	w.lines <- string(data)
	return len(data), nil
}

func TestTailEventsInitialTailAndFollow(t *testing.T) {
	events, _ := testEventTailer(t)
	for i := 0; i < 12; i++ {
		if err := appendEventLine(events.EventFile, fmt.Sprintf(`{"event":%d}`, i)); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	writer := tailedEventWriter{lines: make(chan string, 20)}
	done := make(chan error, 1)
	go func() { done <- events.TailEvents(ctx, writer, 10) }()
	for i := 2; i < 12; i++ {
		expectTailedLine(t, writer.lines, fmt.Sprintf("{\"event\":%d}\n", i))
	}
	if err := appendEventLine(events.FallbackEventFile(), `{"followed":true}`); err != nil {
		t.Fatal(err)
	}
	expectTailedLine(t, writer.lines, "{\"followed\":true}\n")
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("tail exit = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("tail did not stop on cancellation")
	}
	if len(writer.lines) != 0 {
		t.Fatal("tail repeated old events")
	}
}

func expectTailedLine(t *testing.T, lines <-chan string, want string) {
	t.Helper()
	select {
	case got := <-lines:
		if got != want {
			t.Fatalf("line = %q, want %q", got, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %q", want)
	}
}

type failedEventWriter struct{}

func (failedEventWriter) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func TestTailEventsZeroLinesAndErrors(t *testing.T) {
	events, _ := testEventTailer(t)
	if err := appendEventLine(events.EventFile, "{}"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 250*time.Millisecond)
	defer cancel()
	var output bytes.Buffer
	if err := events.TailEvents(ctx, &output, 0); !errors.Is(err, context.DeadlineExceeded) || output.Len() != 0 {
		t.Fatalf("zero lines: output=%q err=%v", output.String(), err)
	}
	if err := events.TailEvents(t.Context(), failedEventWriter{}, 10); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("write error = %v", err)
	}
	if err := events.TailEvents(t.Context(), io.Discard, -1); err == nil {
		t.Fatal("accepted negative line count")
	}
	if err := os.Remove(events.EventFile); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(events.EventFile, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := events.TailEvents(t.Context(), io.Discard, 10); err == nil || !strings.Contains(err.Error(), "regular event file") {
		t.Fatalf("read error = %v", err)
	}
}
