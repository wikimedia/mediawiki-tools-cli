package dockercompose

import (
	"bytes"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func TestContainerStartOutputStreamsIDs(t *testing.T) {
	var completed []string
	output := containerStartOutput{completed: func(id string) { completed = append(completed, id) }}
	_, _ = output.Write([]byte("one\npar"))
	if len(completed) != 1 || completed[0] != "one" {
		t.Fatalf("first completion was buffered: %v", completed)
	}
	_, _ = output.Write([]byte("tial\r\nlast"))
	if len(completed) != 2 || completed[1] != "partial" {
		t.Fatalf("split ID was not completed: %v", completed)
	}
	output.flush()
	output.flush()
	if len(completed) != 3 || completed[2] != "last" || output.output.String() != "one\npartial\r\nlast" {
		t.Fatalf("completed=%v raw=%q", completed, output.output.String())
	}
}

func TestContainerStartProgressPendingCompleteAndFailure(t *testing.T) {
	var buffer bytes.Buffer
	progress := startContainerProgress(&buffer, []string{"one", "two"}, map[string]string{"one": "mwcli-mediawiki-1", "two": "mwcli-mysql-1"}, 80, 24, true)
	progress.complete("unknown-output")
	progress.complete("one")
	progress.finish(errors.New("fixture start failed"))
	text := ansi.Strip(buffer.String())
	for _, want := range []string{"[+] start 0/2", "Starting", "✔ Container mwcli-mediawiki-1", "Started", "✘ Container mwcli-mysql-1", "Error", "[!] start 1/2", "0.0s"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %q", want, text)
		}
	}
	if !strings.Contains(buffer.String(), "\x1b[32mStarted") || !strings.Contains(buffer.String(), "\x1b[31mError") {
		t.Fatal("success and failure colours missing")
	}
}

func TestContainerStartProgressFinalRowsAlignAndFit(t *testing.T) {
	for _, width := range []int{20, 40, 80} {
		var buffer bytes.Buffer
		progress := startContainerProgress(&buffer, []string{"one", "two"}, map[string]string{"one": "a", "two": "very-long-container-name-界-which-must-fit"}, width, 10, false)
		progress.finish(nil)
		if strings.Contains(buffer.String(), "\x1b[32m") {
			t.Fatal("colour emitted when disabled")
		}
		for _, line := range strings.Split(buffer.String(), "\n") {
			if ansi.StringWidth(line) >= width {
				t.Fatalf("line exceeds width %d: %q", width, line)
			}
		}
		// The final redraw has one header and two rows.
		last := buffer.String()[strings.LastIndex(buffer.String(), "\x1b[3A")+5:]
		rows := strings.Split(ansi.Strip(last), "\n")
		if width >= 40 && ansi.StringWidth(strings.Split(rows[1], "Started")[0]) != ansi.StringWidth(strings.Split(rows[2], "Started")[0]) {
			t.Fatalf("status columns not aligned: %q", rows)
		}
	}
}

func TestContainerStartProgressConcurrentCompletions(t *testing.T) {
	var buffer bytes.Buffer
	progress := startContainerProgress(&buffer, []string{"one", "two"}, map[string]string{"one": "mediawiki", "two": "mysql"}, 80, 24, false)
	var workers sync.WaitGroup
	for i := 0; i < 8; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for i := 0; i < 50; i++ {
				progress.complete("one")
			}
		}()
	}
	// Exercise the timer concurrently with completion writes as well.
	time.Sleep(120 * time.Millisecond)
	workers.Wait()
	progress.complete("two")
	progress.finish(nil)
	if !strings.Contains(buffer.String(), "[+] start 2/2") {
		t.Fatal("final completion count missing")
	}
}

func TestContainerStartProgressIgnoresRedirectedOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if progress := newContainerStartProgress(&stdout, &stderr, []string{"one"}, map[string]string{"one": "mediawiki"}); progress != nil {
		progress.finish(nil)
		t.Fatal("redirected output enabled terminal progress")
	}
}
