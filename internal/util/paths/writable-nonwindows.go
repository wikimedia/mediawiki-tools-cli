// +build !windows

package paths

import (
	"errors"
	"os"
	"syscall"
)

func IsWritableDir(path string) (isWritable bool, err error) {
	isWritable = false
	info, err := os.Stat(path)
	if err != nil {
		return false, errors.New("path doesn't exist")
	}

	err = nil
	if !info.IsDir() {
		return false, errors.New("path isn't a directory")
	}

	// Check if the user bit is enabled in file permission
	if info.Mode().Perm()&(1<<(uint(7))) == 0 {
		return false, errors.New("write permission bit is not set on this file for user")
	}

	var stat syscall.Stat_t
	if err = syscall.Stat(path, &stat); err != nil {
		return false, errors.New("unable to get stat")
	}

	err = nil
	if uint32(os.Geteuid()) != stat.Uid {
		isWritable = false
		return false, errors.New("user doesn't have permission to write to this directory")
	}

	return true, nil
}
