package config

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/knadh/koanf"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

func TestGUIStatusAndDisconnectDoNotExposePairingKey(t *testing.T) {
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
	path := secrets.KeePassXCPairingPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	proxy, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("x", 32)))
	data, _ := json.Marshal(map[string]string{"proxy": proxy, "id": "fixture", "key": key, "database_hash": strings.Repeat("a", 64)})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	cmd := Cmd()
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"secrets", "keepassxc", "status"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), path) || strings.Contains(out.String(), key) {
		t.Fatal("status failed to show location safely")
	}
	cmd = Cmd()
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"secrets", "keepassxc", "disconnect"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("disconnect did not remove local pairing")
	}
	if strings.Contains(out.String(), key) {
		t.Fatal("disconnect exposed pairing key")
	}
}

func TestKeePassXCLinkDefaultsToPairedApplication(t *testing.T) {
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
	config.State()
	if err := config.PutDiskConfig(koanf.New(".")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { config.ReApplyKoanfConf(koanf.New(".")) })
	if err := config.PutKeyValueOnDisk("gerrit.username", "fixture"); err != nil {
		t.Fatal(err)
	}
	if err := config.PutKeyValueOnDisk("gerrit.password", "fixture-secret"); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(config.Path())
	if err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	cmd := Cmd()
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"secrets", "use-keepassxc", "gerrit.password"})
	err = cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "config secrets keepassxc connect") {
		t.Fatalf("did not use the paired application: %v", err)
	}
	after, err := os.ReadFile(config.Path())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) || strings.Contains(out.String(), "fixture-secret") {
		t.Fatal("failed pairing lookup modified or exposed plaintext")
	}
}
