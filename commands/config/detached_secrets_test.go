package config

import (
	"bytes"
	"strings"
	"testing"

	"github.com/knadh/koanf"
	"github.com/zalando/go-keyring"
	mwconfig "gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func TestDetachedCommandListsMetadataAndRequiresExplicitDeletion(t *testing.T) {
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
	if err := mwconfig.PutDiskConfig(koanf.New(".")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { mwconfig.ReApplyKoanfConf(koanf.New(".")) })
	keyring.MockInit()
	credential := mwconfig.Credential{Key: "gerrit.profiles.fixture.password", Service: "gerrit", Account: "fixture", Endpoint: "https://gerrit.wikimedia.org/r"}
	if err := credential.SaveWithProvider("private-fixture-secret", false, map[string]string{"gerrit.profiles.fixture.username": "fixture"}, "keyring"); err != nil {
		t.Fatal(err)
	}
	if err := credential.TransferTo("plaintext", mwconfig.TransferOptions{KeepSource: true}); err != nil {
		t.Fatal(err)
	}
	entries, err := mwconfig.DetachedSecrets("")
	if err != nil || len(entries) != 1 {
		t.Fatalf("missing detached entry: %v", err)
	}
	for _, args := range [][]string{{"secrets", "detached", "--details"}, {"secrets", "detached", "--json"}} {
		command := Cmd()
		var output bytes.Buffer
		command.SetOut(&output)
		command.SetErr(&output)
		command.SetArgs(args)
		if err := command.Execute(); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(output.String(), "private-fixture-secret") || !strings.Contains(output.String(), entries[0].ID[:12]) {
			t.Fatal("listing leaked a secret or omitted the ID")
		}
	}
	command := Cmd()
	command.SetOut(&bytes.Buffer{})
	command.SetErr(&bytes.Buffer{})
	command.SetArgs([]string{"secrets", "cleanup", entries[0].ID})
	if err := command.Execute(); err == nil {
		t.Fatal("cleanup ran without explicit approval")
	}
	remaining, err := mwconfig.DetachedSecrets("")
	if err != nil || len(remaining) != 1 {
		t.Fatal("unapproved deletion changed tracking")
	}
}
