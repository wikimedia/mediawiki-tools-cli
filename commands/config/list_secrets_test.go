package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/knadh/koanf"
	"github.com/zalando/go-keyring"
	mwconfig "gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func TestListSecretsTextAndJSONDoNotUnlockOrRevealValues(t *testing.T) {
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
	if err := mwconfig.PutDiskConfig(koanf.New(".")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { mwconfig.ReApplyKoanfConf(koanf.New(".")) })
	keyring.MockInit()
	credential := mwconfig.Credential{Key: "gerrit.password", Service: "gerrit", Endpoint: "https://gerrit.wikimedia.org/r", Account: "Fixture"}
	if err := credential.SaveWithProvider("secret-fixture", false, map[string]string{"gerrit.username": "Fixture"}, "keyring"); err != nil {
		t.Fatal(err)
	}
	if err := mwconfig.PutKeyValueOnDisk("gerrit.password", "newer-plaintext-fixture"); err != nil {
		t.Fatal(err)
	}
	keyring.MockInitWithError(errors.New("locked"))
	for _, mode := range []string{"summary", "details", "json"} {
		asJSON := mode == "json"
		var output bytes.Buffer
		cmd := Cmd()
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		args := []string{"secrets", "list"}
		if asJSON {
			args = append(args, "--json")
		}
		if mode == "details" {
			args = append(args, "--details")
		}
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(output.String(), "secret-fixture") || strings.Contains(output.String(), "newer-plaintext-fixture") {
			t.Fatal("secret value exposed")
		}
		if asJSON {
			var entries []mwconfig.SecretInfo
			if err := json.Unmarshal(output.Bytes(), &entries); err != nil {
				t.Fatal(err)
			}
			if len(entries) != 2 || entries[0].Provider != "keyring" || entries[0].Account != "Fixture" || entries[1].Provider != "plaintext" || entries[1].Source != "plaintext-config" {
				t.Fatal("invalid JSON inventory")
			}
		} else {
			wantCount := 2
			if mode == "details" {
				wantCount = 4
			}
			if strings.Count(output.String(), "gerrit.password") != wantCount || !strings.Contains(output.String(), "not retrieved or verified") {
				t.Fatal("metadata or scope missing")
			}
			if strings.Contains(output.String(), "Credential details:") != (mode == "details") {
				t.Fatal("full details should require --details")
			}
		}
	}
}

func TestSuggestedCommandsStandApartFromExplanations(t *testing.T) {
	var out bytes.Buffer
	if err := printSecretSuggestions(&out, []secretSuggestion{
		{"Keep the valid copy.", "mw config secrets migrate gerrit.password --replace"},
		{"Check authentication.", "mw config secrets list --verify"},
	}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"\n\n  # Keep the valid copy.\n  $ mw config secrets migrate gerrit.password --replace\n",
		"\n\n  # Check authentication.\n  $ mw config secrets list --verify\n",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("command blocks not separated: %q", out.String())
		}
	}
	if strings.Contains(out.String(), "\x1b") {
		t.Fatal("redirected output contains terminal styling")
	}
}

func TestListSecretsCheckJSONReportsDifferencesWithoutValues(t *testing.T) {
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
	if err := mwconfig.PutDiskConfig(koanf.New(".")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { mwconfig.ReApplyKoanfConf(koanf.New(".")) })
	keyring.MockInit()
	credential := mwconfig.Credential{Key: "gerrit.password", Service: "gerrit", Endpoint: "https://gerrit.wikimedia.org/r", Account: "Fixture"}
	if err := credential.SaveWithProvider("vault-private-fixture", false, map[string]string{"gerrit.username": "Fixture"}, "keyring"); err != nil {
		t.Fatal(err)
	}
	if err := mwconfig.PutKeyValueOnDisk(credential.Key, "plaintext-private-fixture"); err != nil {
		t.Fatal(err)
	}
	var output, diagnostics bytes.Buffer
	cmd := Cmd()
	cmd.SetOut(&output)
	cmd.SetErr(&diagnostics)
	cmd.SetArgs([]string{"secrets", "list", "--check", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var entries []mwconfig.SecretInfo
	if err := json.Unmarshal(output.Bytes(), &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Comparison != "different" || entries[1].Comparison != "different" {
		t.Fatal("comparison missing from JSON")
	}
	for _, value := range []string{"vault-private-fixture", "plaintext-private-fixture"} {
		if strings.Contains(output.String()+diagnostics.String(), value) {
			t.Fatal("comparison leaked a secret")
		}
	}
}

func TestListSecretsFiltersBeforeCheckingAndRejectsMissingKeys(t *testing.T) {
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
	if err := mwconfig.PutDiskConfig(koanf.New(".")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { mwconfig.ReApplyKoanfConf(koanf.New(".")) })
	for _, key := range []string{"custom.token", "other.token"} {
		if err := mwconfig.PutKeyValueOnDisk(key, "private-fixture"); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	cmd := Cmd()
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"secrets", "list", "custom.token", "--verify", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var entries []mwconfig.SecretInfo
	if err := json.Unmarshal(out.Bytes(), &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Key != "custom.token" || entries[0].Validation != "unsupported" || strings.Contains(out.String(), "private-fixture") {
		t.Fatal("filter or unsupported status incorrect")
	}
	cmd = Cmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"secrets", "list", "missing.token", "--verify"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("missing key was silently accepted")
	}
}

func TestSecretSuggestionsUseEvidenceBeforeOfferingReplacement(t *testing.T) {
	for _, scenario := range []struct {
		name, comparison, vaultAuth, plainAuth, plainAccount, want string
	}{
		{"valid plaintext", "different", "invalid", "valid", "addshore", "migrate phabricator.sites.wikimedia.key --provider keepassxc --replace"},
		{"identical copies", "match", "valid", "valid", "addshore", "migrate phabricator.sites.wikimedia.key --provider keepassxc"},
		{"unverified difference", "different", "", "", "addshore", "list phabricator.sites.wikimedia.key --check --verify"},
		{"both valid", "different", "valid", "valid", "addshore", ""},
		{"plaintext invalid", "different", "invalid", "invalid", "addshore", ""},
		{"plaintext wrong account", "different", "invalid", "wrong-account", "addshore", ""},
		{"vault unavailable", "different", "unavailable", "valid", "addshore", ""},
		{"different bindings", "different", "invalid", "valid", "Addbot", ""},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			entries := []mwconfig.SecretInfo{
				{Key: "phabricator.sites.wikimedia.key", Source: "credential-reference", Provider: "keepassxc", Account: "addshore", Endpoint: "https://phabricator.wikimedia.org", Comparison: scenario.comparison, Validation: scenario.vaultAuth},
				{Key: "phabricator.sites.wikimedia.key", Source: "plaintext-config", Provider: "plaintext", Account: scenario.plainAccount, Endpoint: "https://phabricator.wikimedia.org", Comparison: scenario.comparison, Validation: scenario.plainAuth},
			}
			got := secretSuggestions(entries, "mw")
			if scenario.want == "" {
				if len(got) != 0 {
					t.Fatalf("unexpected repair suggestion: %+v", got)
				}
			} else if len(got) != 1 || got[0].command != "mw config secrets "+scenario.want {
				t.Fatalf("unexpected suggestion: %+v", got)
			}
		})
	}
}

func TestSecretSuggestionsOfferGerritLoginAndQuoteShellArguments(t *testing.T) {
	got := secretSuggestions([]mwconfig.SecretInfo{{Key: "gerrit.password", Source: "credential-reference", Provider: "keepassxc", Validation: "invalid"}}, "./bin/mw")
	if len(got) != 1 || got[0].command != "./bin/mw gerrit auth login --provider keepassxc" {
		t.Fatalf("missing login suggestion: %+v", got)
	}
	got = secretSuggestions([]mwconfig.SecretInfo{
		{Key: "wiki.accounts.example.$(id).password", Source: "credential-reference", Provider: "keyring", Comparison: "match"},
		{Key: "wiki.accounts.example.$(id).password", Source: "plaintext-config"},
	}, "my cli")
	if len(got) != 1 || got[0].command != "'my cli' config secrets migrate 'wiki.accounts.example.$(id).password' --provider keyring" {
		t.Fatalf("shell metadata not quoted: %+v", got)
	}
}
