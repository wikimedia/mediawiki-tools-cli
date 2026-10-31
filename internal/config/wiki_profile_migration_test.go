package config

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/knadh/koanf"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiauth"
)

func TestWikiProfileMigrationPreservesProviders(t *testing.T) {
	for _, provider := range []string{"keyring", "keepassxc-browser", "plaintext"} {
		t.Run(provider, func(t *testing.T) {
			store := isolatedConfig(t)
			legacy := koanf.New(".")
			oldKey := "wiki.accounts.wikimedia.personal.token"
			newKey := "wiki.profiles.wikimedia.personal.token"
			old := Credential{Key: oldKey, Service: "wiki", Endpoint: wikiauth.WikimediaAuthority, Account: "Addshore"}
			ref, err := old.reference()
			if err != nil {
				t.Fatal(err)
			}
			fields := map[string]string{
				"username": "Addshore", "method": "oauth", "endpoint": wikiauth.WikimediaAuthority, "wiki": wikiauth.WikimediaAuthority + "/w/api.php", "client_id": wikiauth.DefaultClientID, "redirect_uri": wikiauth.DefaultRedirectURI,
			}
			for key, value := range fields {
				_ = legacy.Set("wiki.accounts.wikimedia.personal."+key, value)
			}
			_ = legacy.Set("wiki.realms.wikimedia.default_account", "personal")
			_ = legacy.Set("wiki.default_realm", "wikimedia")
			metadata := map[string]interface{}{"service": ref.Service, "endpoint": ref.Endpoint, "account": ref.Account, "identity": ref.Identity, "provider": provider}
			value := "private-oauth-bundle-fixture"
			switch provider {
			case "keyring":
				store.values[ref.Service+ref.Account] = value
			case "plaintext":
				_ = legacy.Set(oldKey, value)
			case "keepassxc-browser":
				metadata["browser"] = map[string]interface{}{"database_hash": strings.Repeat("a", 64), "entry_id": strings.Repeat("b", 32), "login": "Addshore"}
				previous := readKeePassXCBrowser
				readKeePassXCBrowser = func(ref secrets.KeePassXCBrowser) (string, error) {
					if ref.EntryID != strings.Repeat("b", 32) {
						t.Fatal("entry UUID changed")
					}
					return value, nil
				}
				t.Cleanup(func() { readKeePassXCBrowser = previous })
			}
			_ = legacy.Set(credentialReferenceKey(oldKey), metadata)
			source := legacy.Raw()
			migrated, changed, err := migrateWikiProfiles(legacy)
			if err != nil || !changed {
				t.Fatal("migration failed", err)
			}
			if !reflect.DeepEqual(source, legacy.Raw()) {
				t.Fatal("source mutated")
			}
			if store.reads != 0 {
				t.Fatal("migration read a secret")
			}
			if migrated.Exists("wiki.accounts") || migrated.Exists("wiki.default_realm") || migrated.Exists(credentialReferenceKey(oldKey)) {
				t.Fatal("old config retained")
			}
			if migrated.String("wiki.default_profile") != "wikimedia/personal" || migrated.String("wiki.realms.wikimedia.default_profile") != "personal" {
				t.Fatal("defaults lost")
			}
			if err := PutDiskConfig(migrated); err != nil {
				t.Fatal(err)
			}
			current, err := CredentialForKey(newKey)
			if err != nil {
				t.Fatal(err)
			}
			got, err := current.Read()
			if err != nil || got != value {
				t.Fatal("migrated credential unavailable", err)
			}
			if provider == "keyring" {
				if err := current.SaveWithProvider("refreshed-fixture", false, nil, "keyring"); err != nil {
					t.Fatal(err)
				}
				if store.values[ref.Service+ref.Account] != "refreshed-fixture" {
					t.Fatal("refresh abandoned keyring slot")
				}
			}
			again, changed, err := migrateWikiProfiles(migrated)
			if err != nil || changed || !reflect.DeepEqual(again.Raw(), migrated.Raw()) {
				t.Fatal("migration not idempotent", err)
			}
			if err := current.Delete(); err != nil {
				t.Fatal(err)
			}
			if provider == "keyring" && len(store.values) != 0 {
				t.Fatal("remove abandoned original keyring item")
			}
		})
	}
}

func TestWikiProfileMigrationConflictAndPasswordMethod(t *testing.T) {
	isolatedConfig(t)
	source := koanf.New(".")
	_ = source.Set("wiki.accounts.local.personal.username", "Old")
	_ = source.Set("wiki.profiles.local.personal.username", "New")
	before := source.Raw()
	if _, _, err := migrateWikiProfiles(source); err == nil {
		t.Fatal("conflict accepted")
	}
	if !reflect.DeepEqual(before, source.Raw()) {
		t.Fatal("conflict changed source")
	}
	source = koanf.New(".")
	_ = source.Set("wiki.accounts.local.personal.method", "bot-password")
	migrated, changed, err := migrateWikiProfiles(source)
	if err != nil || !changed || migrated.String("wiki.profiles.local.personal.method") != "password" {
		t.Fatal("password method not migrated", err)
	}
}

func TestWikiProfileMigrationOnLoadAndCompletion(t *testing.T) {
	for _, completion := range []bool{false, true} {
		name := "normal"
		if completion {
			name = "completion"
		}
		t.Run(name, func(t *testing.T) {
			isolatedConfig(t)
			legacy := koanf.New(".")
			_ = legacy.Set("wiki.accounts.wikimedia.personal.username", "Addshore")
			if err := putDiskConfig(legacy); err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(Path())
			if completion {
				old := os.Args
				os.Args = []string{"mw", "__completeNoDesc", "wiki", "--profile", ""}
				t.Cleanup(func() { os.Args = old })
			}
			kLoaded = false
			k, kOnDisk = koanf.New("."), koanf.New(".")
			if State().OnDiskKoanf.String("wiki.profiles.wikimedia.personal.username") != "Addshore" {
				t.Fatal("startup did not translate metadata")
			}
			after, _ := os.ReadFile(Path())
			if completion && string(before) != string(after) {
				t.Fatal("completion wrote config")
			}
			if !completion && string(before) == string(after) {
				t.Fatal("startup did not persist migration")
			}
		})
	}
}
