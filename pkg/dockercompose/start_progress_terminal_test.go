//go:build !windows

package dockercompose

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/creack/pty"
)

type terminalStartRecording struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (r *terminalStartRecording) Write(data []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buffer.Write(data)
}

func (r *terminalStartRecording) text() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buffer.String()
}

func TestStartContainersTerminalStreamsCompletions(t *testing.T) {
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("streaming fixture requires sleep")
	}
	project, _ := startContainersFixture(t)
	t.Setenv("TERM", "xterm")
	t.Setenv("NO_COLOR", "1")
	t.Setenv("MWCLI_START_SLEEP", sleep)
	t.Setenv("MWCLI_START_LABELS", "id-wiki mediawiki /mwcli-test-mediawiki-1\nid-web mediawiki-web /mwcli-test-mediawiki-web-1\nid-mysql mysql /mwcli-test-mysql-1\n")
	t.Setenv("MWCLI_START_FIRST", "id-wiki\n")
	t.Setenv("MWCLI_START_OUTPUT", "id-web\nid-mysql\n")
	release := filepath.Join(t.TempDir(), "release")
	t.Setenv("MWCLI_START_RELEASE", release)
	master, slave, err := pty.Open()
	if err != nil {
		t.Skipf("pseudo-terminal unavailable: %v", err)
	}
	defer master.Close()
	defer slave.Close()
	if err := pty.Setsize(slave, &pty.Winsize{Rows: 24, Cols: 100}); err != nil {
		t.Fatal(err)
	}
	var recording terminalStartRecording
	go func() { _, _ = io.Copy(&recording, master) }()
	done := make(chan error, 1)
	go func() {
		done <- project.StartContainersWithOutput([]string{"mediawiki", "mediawiki-web", "mysql"}, slave, slave)
		close(done)
	}()
	defer func() {
		_ = os.WriteFile(release, nil, 0o600)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("mock Docker start did not exit")
		}
	}()
	waitFor := func(want string) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for !strings.Contains(recording.text(), want) {
			if time.Now().After(deadline) {
				t.Fatalf("terminal never displayed %q: %q", want, recording.text())
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	// This completion must render while the fake Docker process is still blocked.
	waitFor("[+] start 1/3")
	select {
	case err := <-done:
		t.Fatalf("process completed before release: %v", err)
	default:
	}
	if err := os.WriteFile(release, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("mock Docker start did not complete")
	}
	waitFor("[+] start 3/3")
	text := recording.text()
	if strings.Contains(text, "id-wiki") || strings.Contains(text, "Started:") || strings.Contains(text, "\x1b[32m") {
		t.Fatalf("raw IDs, redundant summary or colour leaked: %q", text)
	}
	for _, name := range []string{"mwcli-test-mediawiki-1", "mwcli-test-mediawiki-web-1", "mwcli-test-mysql-1"} {
		if !strings.Contains(ansi.Strip(text), name) {
			t.Fatalf("container name missing: %s", name)
		}
	}
}

func TestContainerStartProgressDumbTerminal(t *testing.T) {
	master, slave, err := pty.Open()
	if err != nil {
		t.Skipf("pseudo-terminal unavailable: %v", err)
	}
	defer master.Close()
	defer slave.Close()
	t.Setenv("TERM", "dumb")
	if progress := newContainerStartProgress(slave, slave, []string{"one"}, map[string]string{"one": "mediawiki"}); progress != nil {
		progress.finish(nil)
		t.Fatal("dumb terminal enabled animated output")
	}
}
