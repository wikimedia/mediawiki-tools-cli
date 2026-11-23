package devenv

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestTraefikSessionKeySurvivesUpgradeAndRestart(t *testing.T) {
	dir := t.TempDir()
	credentials := []byte("Username: existing\nPassword: preserved\n")
	if err := os.WriteFile(filepath.Join(dir, "credentials"), credentials, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ensureTraefikSessionKey(dir); err != nil {
		t.Fatal(err)
	}
	key, err := os.ReadFile(filepath.Join(dir, "session-key"))
	if err != nil {
		t.Fatal(err)
	}
	if len(key) != 32 {
		t.Fatalf("key length: %d", len(key))
	}
	info, err := os.Stat(filepath.Join(dir, "session-key"))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("key permissions: %v", info.Mode())
	}
	if err := ensureTraefikSessionKey(dir); err != nil {
		t.Fatal(err)
	}
	saved, _ := os.ReadFile(filepath.Join(dir, "session-key"))
	if !bytes.Equal(key, saved) {
		t.Fatal("signing key changed on restart")
	}
	saved, _ = os.ReadFile(filepath.Join(dir, "credentials"))
	if !bytes.Equal(credentials, saved) {
		t.Fatal("existing credentials changed")
	}
	if err := os.WriteFile(filepath.Join(dir, "session-key"), []byte("invalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ensureTraefikSessionKey(dir); err == nil {
		t.Fatal("invalid existing key accepted")
	}
}

func TestTraefikAuthInitializesContexts(t *testing.T) {
	previousContext := Context
	Context = "review"
	t.Cleanup(func() { Context = previousContext })
	for _, existing := range []bool{false, true} {
		name := "fresh"
		if existing {
			name = "upgrade"
		}
		t.Run(name, func(t *testing.T) {
			m := Environment{BaseDir: t.TempDir()}
			m.EnsureReady()
			contents, err := m.DockerCompose().File("traefik").Contents()
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := contents.Services["gateway-auth"]; !ok {
				t.Fatal("embedded gateway manifest cannot be inspected")
			}
			authDir := filepath.Join(m.Directory(), "compose", "traefik", "auth")
			if err := os.MkdirAll(authDir, 0o700); err != nil {
				t.Fatal(err)
			}
			oldCredentials := []byte("Username: existing\nPassword: preserved\n")
			if existing {
				if err := os.WriteFile(filepath.Join(authDir, "users"), []byte("existing:legacy-hash\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(authDir, "credentials"), oldCredentials, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var stderr bytes.Buffer
			if err := m.ensureTraefikAuth(&stderr); err != nil {
				t.Fatal(err)
			}
			credentials, err := os.ReadFile(filepath.Join(authDir, "credentials"))
			if err != nil {
				t.Fatal(err)
			}
			if existing && !bytes.Equal(credentials, oldCredentials) {
				t.Fatal("upgrade replaced existing credentials")
			}
			if !existing && !bytes.HasPrefix(credentials, []byte("Username: wm\n")) {
				t.Fatal("fresh gateway uses the legacy username")
			}
			if !existing && !strings.Contains(stderr.String(), string(bytes.TrimSpace(credentials))) {
				t.Fatal("generated credentials were not written to the supplied stderr")
			}
			key, err := os.ReadFile(filepath.Join(authDir, "session-key"))
			if err != nil || len(key) != 32 {
				t.Fatalf("session key not initialized: %v", err)
			}
			if err := m.ensureTraefikAuth(io.Discard); err != nil {
				t.Fatal(err)
			}
			saved, err := os.ReadFile(filepath.Join(authDir, "session-key"))
			if err != nil || !bytes.Equal(key, saved) {
				t.Fatal("restart replaced signing key")
			}
		})
	}
}
