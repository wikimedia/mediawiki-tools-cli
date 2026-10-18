package mwdd

import (
	"bytes"
	"os"
	"path/filepath"
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
	if info.Mode().Perm() != 0o600 {
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

func TestPrepareGatewayInitializesAndMigratesContexts(t *testing.T) {
	previousContext := Context
	Context = "review"
	t.Cleanup(func() { Context = previousContext })
	for _, existing := range []bool{false, true} {
		name := "fresh"
		if existing {
			name = "upgrade"
		}
		t.Run(name, func(t *testing.T) {
			m := MWDD{BaseDir: t.TempDir()}
			m.EnsureReady()
			contents := m.DockerCompose().File("traefik").Contents()
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
			bin := t.TempDir()
			logPath := filepath.Join(t.TempDir(), "docker-args")
			script := `#!/bin/sh
printf '%s\n' "$*" >> "$GATEWAY_TEST_LOG"
if [ "$1" = ps ]; then printf '%s\n' legacy-proxy; fi
`
			if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin)
			t.Setenv("GATEWAY_TEST_LOG", logPath)
			if err := m.PrepareGateway(); err != nil {
				t.Fatal(err)
			}
			credentials, err := os.ReadFile(filepath.Join(authDir, "credentials"))
			if err != nil {
				t.Fatal(err)
			}
			if existing && !bytes.Equal(credentials, oldCredentials) {
				t.Fatal("upgrade replaced existing credentials")
			}
			key, err := os.ReadFile(filepath.Join(authDir, "session-key"))
			if err != nil || len(key) != 32 {
				t.Fatalf("session key not initialized: %v", err)
			}
			if err := m.PrepareGateway(); err != nil {
				t.Fatal(err)
			}
			saved, err := os.ReadFile(filepath.Join(authDir, "session-key"))
			if err != nil || !bytes.Equal(key, saved) {
				t.Fatal("restart replaced signing key")
			}
			args, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(args, []byte("label=com.docker.compose.project=mwcli-mwdd-review")) || !bytes.Contains(args, []byte("label=com.docker.compose.service=nginx-proxy")) || !bytes.Contains(args, []byte("rm --force legacy-proxy")) {
				t.Fatalf("legacy proxy removal not scoped to context: %s", args)
			}
		})
	}
}
