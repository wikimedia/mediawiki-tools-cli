package devremote

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

//go:embed remove.sh
var removeScript string

func startLockPath(name string) string {
	directory, _ := contextDirectory(name)
	return filepath.Join(filepath.Dir(directory), ".start-"+name+".lock")
}

// RemoveContext disconnects and forgets a context. Remote deletion is explicit;
// if teardown fails, its registration remains available for recovery.
func RemoveContext(ctx context.Context, name string, deleteWorkspace bool, stdout, stderr io.Writer) error {
	if _, err := LoadContext(name); err != nil {
		return err
	}
	// Interrupt an in-progress open before waiting for its startup lock.
	lock := flock.New(startLockPath(name))
	deadline := time.NewTimer(30 * time.Second)
	defer deadline.Stop()
	for {
		_ = Disconnect(ctx, name)
		locked, err := lock.TryLock()
		if err != nil {
			return err
		}
		if locked {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return errors.New("context startup is still busy; retry removal")
		case <-time.After(100 * time.Millisecond):
		}
	}
	defer lock.Unlock()
	c, err := LoadContext(name)
	if err != nil {
		return err
	}
	if err := stopSession(ctx, name); err != nil {
		return err
	}
	if deleteWorkspace && c.Workspace != "" {
		if c.Metadata != c.Workspace+"/.wm" {
			return errors.New("refusing to delete an unfamiliar remote workspace layout")
		}
		t := transport{args: c.SSHArgs, stderr: stderr}
		if err := t.script(ctx, removeScript, []string{c.Context, c.Workspace, c.Metadata}, stdout); err != nil {
			return fmt.Errorf("remote cleanup failed; context retained so you can retry: %w", err)
		}
	}
	directory, _ := contextDirectory(name)
	waitContext, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	sessionLock := flock.New(filepath.Join(directory, "session.lock"))
	locked, err := sessionLock.TryLockContext(waitContext, 100*time.Millisecond)
	if err != nil {
		return err
	}
	if !locked {
		return errors.New("session has not finished shutting down")
	}
	sessionLock.Unlock()
	if err := os.RemoveAll(directory); err != nil {
		return err
	}
	return nil
}

func stopSession(ctx context.Context, name string) error {
	if controlRequest(ctx, name, "/status", nil) != nil {
		return nil
	}
	if err := Disconnect(ctx, name); err != nil {
		return err
	}
	for i := 0; i < 150; i++ {
		if controlRequest(ctx, name, "/status", nil) != nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return errors.New("session is still shutting down; retry removal")
}
