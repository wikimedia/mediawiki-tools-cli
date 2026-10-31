package config

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/knadh/koanf"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

func TestGerritProfileMigrationPreservesProviders(t *testing.T) {
	for _, provider := range []string{"keyring", "keepassxc-browser", "plaintext"} {
		t.Run(provider, func(t *testing.T) {
			store := isolatedConfig(t)
			cred := Credential{Key: "gerrit.password", Service: "gerrit", Endpoint: secrets.GerritEndpoint, Account: "Addshore"}
			ref, err := cred.reference()
			if err != nil {
				t.Fatal(err)
			}
			legacy := koanf.New(".")
			_ = legacy.Set("gerrit.username", "Addshore")
			_ = legacy.Set("gerrit.interaction_type", "ssh")
			metadata := map[string]interface{}{"service": ref.Service, "endpoint": ref.Endpoint, "account": ref.Account, "identity": ref.Identity, "provider": provider}
			value := "private-gerrit-migration-fixture"
			switch provider {
			case "keyring":
				store.values[ref.Service+ref.Account] = value
			case "plaintext":
				_ = legacy.Set("gerrit.password", value)
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
			_ = legacy.Set(credentialReferenceKey("gerrit.password"), metadata)
			source := legacy.Raw()
			migrated, changed, err := migrateGerritProfiles(legacy)
			if err != nil || !changed {
				t.Fatalf("migration failed: %v", err)
			}
			if !reflect.DeepEqual(source, legacy.Raw()) {
				t.Fatal("source mutated")
			}
			if store.reads != 0 {
				t.Fatal("migration accessed credential provider")
			}
			if migrated.Exists("gerrit.username") || migrated.Exists("gerrit.password") || migrated.Exists(credentialReferenceKey("gerrit.password")) {
				t.Fatal("legacy keys retained")
			}
			if migrated.String("gerrit.default_profile") != "addshore" || migrated.String("gerrit.interaction_type") != "ssh" {
				t.Fatal("default/SSH settings lost")
			}
			if err := PutDiskConfig(migrated); err != nil {
				t.Fatal(err)
			}
			current, err := CredentialForKey("gerrit.profiles.addshore.password")
			if err != nil {
				t.Fatal(err)
			}
			got, err := current.Read()
			if err != nil || got != value {
				t.Fatal("migrated credential unavailable", err)
			}
			if State().Effective.Gerrit.Username != "Addshore" {
				t.Fatal("default username unavailable to cloning commands")
			}
			if provider == "keyring" {
				if err := current.SaveWithProvider("updated-fixture", false, nil, "keyring"); err != nil {
					t.Fatal(err)
				}
				if store.values[ref.Service+ref.Account] != "updated-fixture" {
					t.Fatal("save changed external keyring slot")
				}
			}
			again, changed, err := migrateGerritProfiles(migrated)
			if err != nil || changed || !reflect.DeepEqual(again.Raw(), migrated.Raw()) {
				t.Fatal("migration is not idempotent", err)
			}
			if err := current.Delete(); err != nil {
				t.Fatal(err)
			}
			if provider == "keyring" && len(store.values) != 0 {
				t.Fatal("remove abandoned keyring item")
			}
		})
	}
}

func TestGerritProfileMigrationOnLoad(t *testing.T) {
	isolatedConfig(t)
	legacy := koanf.New(".")
	_ = legacy.Set("gerrit.username", "fixture")
	_ = legacy.Set("gerrit.password", "private-fixture")
	if err := putDiskConfig(legacy); err != nil {
		t.Fatal(err)
	}
	kLoaded = false
	k, kOnDisk = koanf.New("."), koanf.New(".")
	if State().OnDiskKoanf.String("gerrit.profiles.fixture.password") != "private-fixture" {
		t.Fatal("startup did not migrate credentials")
	}
	data, err := os.ReadFile(Path())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"profiles"`) {
		t.Fatal("startup did not persist migration")
	}
}

func TestGerritProfileMigrationEmptyFieldsAndConflict(t *testing.T) {
	isolatedConfig(t)
	empty := koanf.New(".")
	_ = empty.Set("gerrit.username", "")
	_ = empty.Set("gerrit.password", "")
	migrated, _, err := migrateGerritProfiles(empty)
	if err != nil || migrated.Exists("gerrit.username") || migrated.Exists("gerrit.default_profile") {
		t.Fatal("empty credentials became an unusable profile", err)
	}
	legacy := koanf.New(".")
	_ = legacy.Set("gerrit.username", "fixture")
	_ = legacy.Set("gerrit.profiles.fixture.username", "different")
	before := legacy.Raw()
	if _, _, err := migrateGerritProfiles(legacy); err == nil {
		t.Fatal("conflict accepted")
	}
	if !reflect.DeepEqual(before, legacy.Raw()) {
		t.Fatal("conflict changed source")
	}
}
