package config

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/knadh/koanf"
	"github.com/zalando/go-keyring"
	mwconfig "gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func TestReplaceMigrationRequiresOneKeyAndDryRunDoesNotWrite(t *testing.T) {
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
	if err := mwconfig.PutDiskConfig(koanf.New(".")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { mwconfig.ReApplyKoanfConf(koanf.New(".")) })
	keyring.MockInit()
	credential := mwconfig.Credential{Key: "phabricator.sites.wikimedia.key", Service: "phabricator", Endpoint: "https://phabricator.wikimedia.org", Account: "Fixture"}
	if err := credential.SaveWithProvider("old-provider-fixture", false, map[string]string{"phabricator.sites.wikimedia.username": "Fixture", "phabricator.sites.wikimedia.url": "https://phabricator.wikimedia.org"}, "keyring"); err != nil {
		t.Fatal(err)
	}
	if err := mwconfig.PutKeyValueOnDisk(credential.Key, "new-plaintext-fixture"); err != nil {
		t.Fatal(err)
	}
	for _, keys := range [][]string{nil, {credential.Key, "gerrit.password"}} {
		cmd := Cmd()
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs(append([]string{"secrets", "migrate", "--replace"}, keys...))
		if err := cmd.Execute(); err == nil {
			t.Fatal("replacement accepted an implicit or multiple-key selection")
		}
	}
	var out bytes.Buffer
	cmd := Cmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"secrets", "migrate", credential.Key, "--replace", "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got, _ := credential.Read(); got != "old-provider-fixture" || mwconfig.State().OnDiskKoanf.String(credential.Key) != "new-plaintext-fixture" {
		t.Fatal("dry run modified credential")
	}
	cmd = Cmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"secrets", "migrate", credential.Key, "--replace"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got, _ := credential.Read(); got != "new-plaintext-fixture" || mwconfig.State().OnDiskKoanf.Exists(credential.Key) {
		t.Fatal("replacement did not move plaintext")
	}
	if strings.Contains(out.String(), "old-provider-fixture") || strings.Contains(out.String(), "new-plaintext-fixture") {
		t.Fatal("migration exposed secret values")
	}
	encoded, _ := json.Marshal(mwconfig.SecretInventory())
	if bytes.Contains(encoded, []byte("new-plaintext-fixture")) {
		t.Fatal("inventory exposed replacement")
	}
}
