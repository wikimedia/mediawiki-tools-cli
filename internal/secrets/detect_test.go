package secrets

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestKeePassXCDiscoveryDoesNotContactGUI(t *testing.T) {
	p := browserFixture(t, "success")
	if err := saveBrowserPairing(p); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(t.TempDir(), "actions")
	t.Setenv("WM_BROWSER_ACTION_LOG", log)
	got := DetectKeePassXC()
	if !got.Available || got.NeedsPairing {
		t.Fatal("valid pairing was not detected")
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatal("discovery contacted proxy")
	}
}

func TestKeePassXCDiscoveryFindsUnpairedInstallation(t *testing.T) {
	browserFixture(t, "success")
	directory := t.TempDir()
	name := "keepassxc-proxy"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := os.WriteFile(filepath.Join(directory, name), []byte("must never be executed"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory)
	got := DetectKeePassXC()
	if !got.Available || !got.NeedsPairing {
		t.Fatal("did not detect unpaired installation")
	}
}

func TestKeePassXCDiscoveryWithoutInstallation(t *testing.T) {
	browserFixture(t, "success")
	t.Setenv("PATH", t.TempDir())
	if got := DetectKeePassXC(); got.Available {
		t.Fatal("reported missing installation as available")
	}
}
