//go:build unix

package diskcache

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// Keep the lock file inode stable: unlinking it would let two processes lock
// different files at the same path. Closing the descriptor releases ownership.
func tryLockFile(file *os.File) (bool, error) {
	err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
		return false, nil
	}
	return err == nil, err
}
