package config

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/knadh/koanf"
	"github.com/zalando/go-keyring"
	mwconfig "gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func TestMainPlaintextConfigurationMigratesThroughSecretstore(t *testing.T) {
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
	disk := koanf.New(".")
	values := map[string]string{
		"gerrit.username": "fixture", "gerrit.password": "gerrit-private-fixture",
		"cloud_vps.projects.fixture.credential.id": "application-id", "cloud_vps.projects.fixture.credential.secret": "cloud-private-fixture",
		"phabricator.sites.wikimedia.url": "https://phabricator.wikimedia.org", "phabricator.sites.wikimedia.username": "fixture", "phabricator.sites.wikimedia.key": "phab-private-fixture",
	}
	for key, value := range values {
		if err := disk.Set(key, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := mwconfig.PutDiskConfig(disk); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { mwconfig.ReApplyKoanfConf(koanf.New(".")) })
	keyring.MockInit()
	var output bytes.Buffer
	cmd := Cmd()
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"secrets", "migrate", "--provider", "keyring"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"gerrit.password", "cloud_vps.projects.fixture.credential.secret", "phabricator.sites.wikimedia.key"} {
		if mwconfig.State().OnDiskKoanf.Exists(key) {
			t.Fatalf("plaintext retained for %s", key)
		}
		credential, err := mwconfig.CredentialForKey(key)
		if err != nil {
			t.Fatal(err)
		}
		if got, err := credential.Read(); err != nil || got != values[key] {
			t.Fatalf("migrated credential cannot be read for %s", key)
		}
	}
	data, err := os.ReadFile(mwconfig.Path())
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range values {
		if mwconfig.SensitiveKey(key) && strings.Contains(string(data)+output.String(), value) {
			t.Fatal("migration exposed a secret")
		}
	}
}
