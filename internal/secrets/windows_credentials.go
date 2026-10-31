package secrets

import (
	"context"
	"fmt"
	"io"
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
func WindowsCredentialStore(helper string) (Store, error) {
	if runtime.GOOS == "windows" {
		return Keyring{}, nil
	}
	if !IsWSL() {
		return nil, fmt.Errorf("Windows Credential Manager requires Windows or WSL")
	}
	path, err := WindowsHelperPath(helper)
	if err != nil {
		return nil, err
	}
	return &native.BridgeClient{Executable: path, Args: []string{"__credential-helper"}, ProbeService: CredentialServicePrefix + "credential-store-probe"}, nil
}

func WindowsHelperPath(configured string) (string, error) {
	candidates := []string{configured}
	if configured == "" {
		if executable, err := os.Executable(); err == nil {
			candidates = []string{filepath.Join(filepath.Dir(executable), "mw.exe")}
		}
		candidates = append(candidates, "mw.exe")
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		path, err := exec.LookPath(candidate)
		if err == nil && strings.EqualFold(filepath.Ext(path), ".exe") {
			return filepath.Abs(path)
		}
	}
	return "", fmt.Errorf("Windows credential helper not found; place mw.exe beside mw or set secrets.windows_helper to its absolute WSL path")
}

func DetectWindowsCredentials(ctx context.Context, helper string) Availability {
	if runtime.GOOS == "windows" {
		return DetectKeyring(ctx)
	}
	store, err := WindowsCredentialStore(helper)
	if err != nil {
		return Availability{Detail: err.Error()}
	}
	bridge, ok := store.(*native.BridgeClient)
	if !ok || bridge.Probe(ctx) != nil {
		return Availability{Detail: "Windows helper could not access Credential Manager; check WSL interoperability and the helper version"}
	}
	return Availability{Available: true, Detail: "Windows Credential Manager via " + bridge.Executable}
}

// The private entry point deliberately bypasses config, telemetry and timers.
// Enumeration is restricted to CLI-owned namespaces; unrelated credentials cannot be addressed.
func ServeWindowsCredentials(input io.Reader, output io.Writer) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("credential helper must run as a Windows executable")
	}
	return native.ServeBridge(input, output, Keyring{}, OwnsCredentialService)
}
