package secrets

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	native "gitlab.wikimedia.org/repos/releng/cli/pkg/secretstore/keyring"
)

const WindowsProvider = native.WindowsProvider

func IsWSL() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	if os.Getenv("WSL_INTEROP") != "" || os.Getenv("WSL_DISTRO_NAME") != "" {
		return true
	}
	data, _ := os.ReadFile("/proc/sys/kernel/osrelease")
	return strings.Contains(strings.ToLower(string(data)), "microsoft")
}

// WindowsCredentialStore never silently falls back to Linux Secret Service.
func WindowsCredentialStore() (Store, error) {
	if runtime.GOOS == "windows" {
		return Keyring{}, nil
	}
	if !IsWSL() {
		return nil, fmt.Errorf("Windows Credential Manager requires Windows or WSL")
	}
	path, err := WindowsPowerShellPath()
	if err != nil {
		return nil, err
	}
	return native.NewPowerShellBridge(path, CredentialNamespace)
}

// WindowsPowerShellPath finds the system Windows PowerShell, including when WSL
// does not append the Windows PATH. No downloaded executable or module is used.
func WindowsPowerShellPath() (string, error) {
	for _, candidate := range []string{"powershell.exe", "/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe"} {
		if path, err := exec.LookPath(candidate); err == nil {
			return filepath.Abs(path)
		}
	}
	return "", fmt.Errorf("Windows PowerShell not found; enable WSL interoperability and make powershell.exe accessible")
}

func DetectWindowsCredentials(ctx context.Context) Availability {
	if runtime.GOOS == "windows" {
		return DetectKeyring(ctx)
	}
	store, err := WindowsCredentialStore()
	if err != nil {
		return Availability{Detail: err.Error()}
	}
	bridge, ok := store.(*native.BridgeClient)
	if !ok || bridge.Probe(ctx) != nil {
		return Availability{Detail: "Windows PowerShell could not access Credential Manager; check WSL interoperability, PowerShell policy and the Windows logon session"}
	}
	return Availability{Available: true, Detail: "Windows Credential Manager via PowerShell: " + bridge.Executable}
}
