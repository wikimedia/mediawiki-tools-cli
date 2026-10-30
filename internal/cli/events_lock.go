package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var errEventsLockTimeout = errors.New("timed out waiting for telemetry lock")

// acquireEventsLock uses an atomic directory creation as a portable lock for
// processes sharing the same temporary event directory.
func acquireEventsLock(path string, wait time.Duration) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(wait)
	for {
		if err := os.Mkdir(path, 0o700); err == nil {
			return func() { _ = os.Remove(path) }, nil
		} else if !os.IsExist(err) {
			return nil, err
		}

		if info, err := os.Stat(path); err == nil && time.Since(info.ModTime()) > eventLockStaleAge {
			stalePath := fmt.Sprintf("%s.stale-%d", path, time.Now().UnixNano())
			if err := os.Rename(path, stalePath); err == nil {
				_ = os.RemoveAll(stalePath)
				continue
			}
		}
		if wait <= 0 || !time.Now().Before(deadline) {
			return nil, fmt.Errorf("%w: %s", errEventsLockTimeout, path)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
