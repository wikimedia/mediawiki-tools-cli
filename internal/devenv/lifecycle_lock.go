package devenv

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

// WithServiceLifecycle serializes lifecycle commands within one context. OS locks
// are released when a process exits, including after an interrupted Docker call.
func (m Environment) WithServiceLifecycle(ctx context.Context, action func() error) error {
	return m.withServiceLock(ctx, ".service-lifecycle.lock", action)
}

func (m Environment) withServiceLock(ctx context.Context, name string, action func() error) (err error) {
	if err := os.MkdirAll(m.Directory(), 0o700); err != nil {
		return err
	}
	lock := flock.New(filepath.Join(m.Directory(), name))
	locked, err := lock.TryLockContext(ctx, 100*time.Millisecond)
	if err != nil {
		return fmt.Errorf("lock service state: %w", err)
	}
	if !locked {
		return fmt.Errorf("service state lock was not acquired")
	}
	defer func() { err = errors.Join(err, lock.Close()) }()
	return action()
}
