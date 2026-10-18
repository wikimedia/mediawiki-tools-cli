//go:build !linux

package secrets

import (
	"context"
	"os"
	"runtime"
)

func DetectKeyring(context.Context) Availability {
	switch runtime.GOOS {
	case "windows":
		return Availability{Available: true, Detail: "Windows Credential Manager"}
	case "darwin":
		if _, err := os.Stat("/usr/bin/security"); err == nil {
			return Availability{Available: true, Detail: "macOS Keychain; unlocking may be required"}
		}
	}
	return Availability{Detail: "no supported OS keyring detected"}
}
