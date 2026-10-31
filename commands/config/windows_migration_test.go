package config

import (
	"bytes"
	"strings"
	"testing"

	"github.com/knadh/koanf"
	"github.com/zalando/go-keyring"
	mwconfig "gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

func TestWindowsReplacementAndSuggestions(t *testing.T) {
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
	disk := koanf.New(".")
	for key, value := range map[string]string{"gerrit.profiles.fixture.username": "fixture", "gerrit.profiles.fixture.password": "private-fixture"} {
		if err := disk.Set(key, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := mwconfig.PutDiskConfig(disk); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { mwconfig.ReApplyKoanfConf(koanf.New(".")) })
	var output bytes.Buffer
	cmd := Cmd()
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"secrets", "migrate", "gerrit.profiles.fixture.password", "--replace", "--provider", secrets.WindowsProvider, "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Windows Credential Manager") || strings.Contains(output.String(), "private-fixture") {
		t.Fatal("replacement preview is missing or exposes plaintext")
	}
	entries := []mwconfig.SecretInfo{
		{Key: "gerrit.profiles.fixture.password", Source: "credential-reference", Provider: secrets.WindowsProvider, Service: "gerrit", Account: "fixture", Endpoint: secrets.GerritEndpoint, Comparison: "match"},
		{Key: "gerrit.profiles.fixture.password", Source: "plaintext-config", Provider: "plaintext", Service: "gerrit", Account: "fixture", Endpoint: secrets.GerritEndpoint, Comparison: "match"},
	}
	suggestions := secretSuggestions(entries, "mw")
	if len(suggestions) != 1 || !strings.Contains(suggestions[0].command, "--provider "+secrets.WindowsProvider) {
		t.Fatal("Windows duplicate cleanup suggestion missing")
	}
	entries[0].Comparison = "different"
	entries[0].Validation = "invalid"
	entries[1].Validation = "valid"
	suggestions = secretSuggestions(entries, "mw")
	if len(suggestions) != 1 || !strings.Contains(suggestions[0].command, "--replace") {
		t.Fatal("Windows replacement suggestion missing")
	}
}

func TestTransferReportsRetainedPlaintext(t *testing.T) {
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
	if err := mwconfig.PutDiskConfig(koanf.New(".")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { mwconfig.ReApplyKoanfConf(koanf.New(".")) })
	keyring.MockInit()
	cred := mwconfig.Credential{Key: "gerrit.profiles.fixture.password", Service: "gerrit", Endpoint: secrets.GerritEndpoint, Account: "fixture"}
	if err := cred.SaveWithProvider("private-fixture", false, map[string]string{"gerrit.profiles.fixture.username": "fixture"}, "plaintext"); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	cmd := NewTransferSecretCmd()
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{cred.Key, "--provider", "keyring", "--keep-source"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Plaintext source retained") || strings.Contains(output.String(), "detached") || strings.Contains(output.String(), "private-fixture") {
		t.Fatal("incorrect plaintext retention report")
	}
	if mwconfig.State().OnDiskKoanf.String(cred.Key) != "private-fixture" {
		t.Fatal("plaintext was removed")
	}
}
