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
