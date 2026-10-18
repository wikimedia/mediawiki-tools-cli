package config

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

func TestCredentialProviderSelection(t *testing.T) {
	for _, tc := range []struct{ name, defaultProvider, override, want string }{
		{"initial", "", "", ""},
		{"default", "keepassxc", "", "keepassxc"},
		{"override", "keepassxc", "keyring", "keyring"},
		{"inverse", "keyring", "keepassxc", "keepassxc"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolatedConfig(t)
			cred := seedCredential(t)
			if tc.defaultProvider != "" {
				if err := PutKeyValueOnDisk("secrets.provider", tc.defaultProvider); err != nil {
					t.Fatal(err)
				}
			}
			got, err := cred.CredentialProvider(tc.override)
			if err != nil || got != tc.want {
				t.Fatalf("provider: %s, %v", got, err)
			}
		})
	}
}

func TestCredentialKeePassXCDefaultMigration(t *testing.T) {
	store := isolatedConfig(t)
	cred := seedCredential(t)
	if err := PutKeyValueOnDisk("secrets.provider", "keepassxc"); err != nil {
		t.Fatal(err)
	}
	old := writeKeePassXCBrowser
	t.Cleanup(func() { writeKeePassXCBrowser = old })
	calls := 0
	writeKeePassXCBrowser = func(_ context.Context, existing *secrets.KeePassXCBrowser, key, account, endpoint, value string) (secrets.KeePassXCBrowser, error) {
		calls++
		if existing != nil || key != cred.Key || account != cred.Account || endpoint != cred.Endpoint || value != "legacy-secret" {
			t.Fatal("incorrect credential passed to provider")
		}
		return secrets.KeePassXCBrowser{DatabaseHash: strings.Repeat("a", 64), EntryID: strings.Repeat("b", 32), Login: account}, nil
	}
	if err := cred.MigrateTo(""); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(Path())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "legacy-secret") || calls != 1 || len(store.values) != 0 {
		t.Fatal("migration leaked plaintext or used wrong provider")
	}
	if err := PutKeyValueOnDisk("secrets.provider", "keyring"); err != nil {
		t.Fatal(err)
	}
	got, err := cred.CredentialProvider("")
	if got != "keepassxc" || err != nil {
		t.Fatal("changing default changed existing binding")
	}
	if err := cred.SaveWithProvider("other-secret", false, nil, "keyring"); err == nil {
		t.Fatal("silently switched an existing provider")
	}
	if calls != 1 {
		t.Fatal("conflicting override contacted provider")
	}
}

func TestCredentialKeyringOverrideRetainsBinding(t *testing.T) {
	isolatedConfig(t)
	cred := seedCredential(t)
	if err := PutKeyValueOnDisk("secrets.provider", "keepassxc"); err != nil {
		t.Fatal(err)
	}
	if err := cred.SaveWithProvider("vault-secret", false, nil, "keyring"); err != nil {
		t.Fatal(err)
	}
	if err := cred.Save("replacement", false); err != nil {
		t.Fatal(err)
	}
	got, err := cred.Read()
	if err != nil || got != "replacement" {
		t.Fatal("existing keyring binding was not retained")
	}
}

func TestCredentialProviderFailurePreservesPlaintext(t *testing.T) {
	for _, mode := range []string{"failure", "wrong-account", "invalid-provider", "insecure-conflict"} {
		t.Run(mode, func(t *testing.T) {
			isolatedConfig(t)
			cred := seedCredential(t)
			old := writeKeePassXCBrowser
			t.Cleanup(func() { writeKeePassXCBrowser = old })
			writeKeePassXCBrowser = func(context.Context, *secrets.KeePassXCBrowser, string, string, string, string) (secrets.KeePassXCBrowser, error) {
				if mode == "wrong-account" {
					return secrets.KeePassXCBrowser{Login: "other"}, nil
				}
				return secrets.KeePassXCBrowser{}, errors.New("unavailable")
			}
			provider := "keepassxc"
			if mode == "invalid-provider" {
				provider = "invalid"
			}
			if err := cred.SaveWithProvider("replacement", mode == "insecure-conflict", nil, provider); err == nil {
				t.Fatal("expected failure")
			}
			if State().OnDiskKoanf.String(cred.Key) != "legacy-secret" {
				t.Fatal("failed save removed plaintext")
			}
		})
	}
}

func TestCredentialMigrationDoesNotOverwriteExistingProvider(t *testing.T) {
	store := isolatedConfig(t)
	cred := seedCredential(t)
	if err := cred.Save("vault-secret", false); err != nil {
		t.Fatal(err)
	}
	if err := PutKeyValueOnDisk(cred.Key, "stale"); err != nil {
		t.Fatal(err)
	}
	if err := cred.MigrateTo(""); err == nil {
		t.Fatal("accepted mismatching plaintext")
	}
	if State().OnDiskKoanf.String(cred.Key) != "stale" {
		t.Fatal("removed mismatching plaintext")
	}
	for _, value := range store.values {
		if value != "vault-secret" {
			t.Fatal("overwrote provider")
		}
	}
	if err := PutKeyValueOnDisk(cred.Key, "vault-secret"); err != nil {
		t.Fatal(err)
	}
	if err := cred.MigrateTo(""); err != nil {
		t.Fatal(err)
	}
	if State().OnDiskKoanf.Exists(cred.Key) {
		t.Fatal("matching plaintext remains")
	}
}
