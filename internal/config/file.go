package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Reject symlinks rather than following or silently replacing linked config.
// Startup can read an existing config when the filesystem prevents chmod.
func secureConfigPath(requirePrivatePermissions bool) error {
	dir := filepath.Dir(Path())
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("configuration directory must be a real directory: %s", dir)
	}
	// In CI this directory is the checkout, which other containers must read.
	if _, inCI := os.LookupEnv("GITLAB_CI"); !inCI && info.Mode() != os.ModeDir|0o700 {
		if err := os.Chmod(dir, 0o700); err != nil && requirePrivatePermissions {
			return err
		}
	}
	info, err = os.Lstat(Path())
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("configuration must be a regular file, not a symlink: %s", Path())
	}
	if info.Mode() == 0o600 {
		return nil
	}
	if err := os.Chmod(Path(), 0o600); err != nil && requirePrivatePermissions {
		return err
	}
	return nil
}

func writePrivateConfig(data []byte) error {
	if err := secureConfigPath(true); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(Path()), ".config-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), Path())
}
