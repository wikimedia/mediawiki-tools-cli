package config

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/knadh/koanf"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

func TestPhabricatorMigrationPreservesProviders(t *testing.T) {
	for _, provider := range []string{"keyring", "keepassxc-browser", "plaintext"} {
		t.Run(provider, func(t *testing.T) {
			store := isolatedConfig(t)
			oldKey := "phabricator.sites.personal.key"
			newKey := "phabricator.profiles.personal.key"
			legacy := koanf.New(".")
			for key, value := range map[string]string{
				"phabricator.default_site":            "personal",
				"phabricator.sites.personal.username": "Addshore",
				"phabricator.sites.personal.url":      secrets.PhabricatorEndpoint,
				"unrelated.value":                     "retained",
			} {
				if err := legacy.Set(key, value); err != nil {
					t.Fatal(err)
				}
			}
			oldCred := Credential{Key: oldKey, Service: "phabricator", Endpoint: secrets.PhabricatorEndpoint, Account: "Addshore"}
			ref, err := oldCred.reference()
			if err != nil {
				t.Fatal(err)
			}
			metadata := map[string]interface{}{"service": ref.Service, "endpoint": ref.Endpoint, "account": ref.Account, "identity": ref.Identity, "provider": provider}
			value := "migration-private-fixture"
			switch provider {
			case "keyring":
				store.values[ref.Service+ref.Account] = value
			case "plaintext":
				_ = legacy.Set(oldKey, value)
			case "keepassxc-browser":
				metadata["browser"] = map[string]interface{}{"database_hash": strings.Repeat("a", 64), "entry_id": strings.Repeat("b", 32), "login": "Addshore"}
				oldRead := readKeePassXCBrowser
				readKeePassXCBrowser = func(got secrets.KeePassXCBrowser) (string, error) {
					if got.EntryID != strings.Repeat("b", 32) {
						return "", fmt.Errorf("UUID changed")
					}
					return value, nil
				}
				t.Cleanup(func() { readKeePassXCBrowser = oldRead })
			}
			if err := legacy.Set(credentialReferenceKey(oldKey), metadata); err != nil {
				t.Fatal(err)
			}
			before := legacy.Raw()
			migrated, changed, err := migratePhabricatorProfiles(legacy)
			if err != nil || !changed {
				t.Fatalf("migration: changed=%v err=%v", changed, err)
			}
			if !reflect.DeepEqual(before, legacy.Raw()) {
				t.Fatal("migration mutated its input")
			}
			if store.reads != 0 {
				t.Fatal("migration retrieved a secret")
			}
			if migrated.Exists("phabricator.sites") || migrated.Exists("phabricator.default_site") || migrated.Exists(credentialReferenceKey(oldKey)) {
				t.Fatal("legacy keys retained")
			}
			if migrated.String("phabricator.default_profile") != "personal" || migrated.String("unrelated.value") != "retained" {
				t.Fatal("metadata lost")
			}
			if err := PutDiskConfig(migrated); err != nil {
				t.Fatal(err)
			}
			cred, err := CredentialForKey(newKey)
			if err != nil {
				t.Fatal(err)
			}
			got, err := cred.Read()
			if err != nil || got != value {
				t.Fatalf("migrated read: %q %v", got, err)
			}
			if provider == "keyring" {
				if err := cred.SaveWithProvider("updated-fixture", false, nil, "keyring"); err != nil {
					t.Fatal(err)
				}
				if store.values[ref.Service+ref.Account] != "updated-fixture" {
					t.Fatal("save abandoned original keyring slot")
				}
			}
			again, changed, err := migratePhabricatorProfiles(migrated)
			if err != nil || changed || !reflect.DeepEqual(again.Raw(), migrated.Raw()) {
				t.Fatal("migration is not idempotent", err)
			}
			if err := cred.Delete(); err != nil {
				t.Fatal(err)
			}
			if provider == "keyring" && len(store.values) != 0 {
				t.Fatal("logout left keyring credential")
			}
		})
	}
}

func TestPhabricatorMigrationConflictsPreserveSource(t *testing.T) {
	isolatedConfig(t)
	legacy := koanf.New(".")
	_ = legacy.Set("phabricator.sites.personal.username", "Old")
	_ = legacy.Set("phabricator.profiles.personal.username", "New")
	before := legacy.Raw()
	if _, _, err := migratePhabricatorProfiles(legacy); err == nil {
		t.Fatal("conflict accepted")
	}
	if !reflect.DeepEqual(before, legacy.Raw()) {
		t.Fatal("conflict changed source")
	}
}

func TestPhabricatorFlatMigrationOnLoad(t *testing.T) {
	isolatedConfig(t)
	legacy := koanf.New(".")
	for key, value := range map[string]string{
		"phabricator.username": "Addshore", "phabricator.key": "private-fixture", "phabricator.url": secrets.PhabricatorEndpoint,
	} {
		_ = legacy.Set(key, value)
	}
	if err := putDiskConfig(legacy); err != nil {
		t.Fatal(err)
	}
	kLoaded = false
	k, kOnDisk = koanf.New("."), koanf.New(".")
	disk := State().OnDiskKoanf
	if disk.String("phabricator.profiles.wikimedia.key") != "private-fixture" || disk.String("phabricator.default_profile") != "wikimedia" {
		t.Fatal("startup did not migrate flat config")
	}
	data, err := os.ReadFile(Path())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"default_site"`) || disk.Exists("phabricator.key") {
		t.Fatal("legacy config retained")
	}
	// A subsequent load does not rewrite the file.
	before, _ := os.Stat(Path())
	kLoaded = false
	State()
	after, _ := os.Stat(Path())
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("already migrated config was rewritten")
	}
}

func TestPhabricatorCompletionMigrationDoesNotPersist(t *testing.T) {
	isolatedConfig(t)
	legacy := koanf.New(".")
	_ = legacy.Set("phabricator.sites.personal.username", "Addshore")
	if err := putDiskConfig(legacy); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(Path())
	oldArgs := os.Args
	os.Args = []string{"mw", "__completeNoDesc", "phab", "--profile", ""}
	t.Cleanup(func() { os.Args = oldArgs })
	kLoaded = false
	k, kOnDisk = koanf.New("."), koanf.New(".")
	if State().EffectiveKoanf.String("phabricator.profiles.personal.username") != "Addshore" {
		t.Fatal("completion cannot see migrated metadata")
	}
	after, _ := os.ReadFile(Path())
	if string(before) != string(after) {
		t.Fatal("completion wrote migrated config")
	}
}

func TestPhabricatorMigrationRemovesEmptyLogoutMarkers(t *testing.T) {
	isolatedConfig(t)
	legacy := koanf.New(".")
	_ = legacy.Set("phabricator.profiles.wikimedia.logged_out", true)
	_ = legacy.Set("phabricator.profiles.wikimedia.url", secrets.PhabricatorEndpoint)
	_ = legacy.Set("phabricator.default_profile", "wikimedia")
	_ = legacy.Set("phabricator.profiles.retained.logged_out", true)
	_ = legacy.Set("phabricator.profiles.retained.key", "private-fixture")
	migrated, changed, err := migratePhabricatorProfiles(legacy)
	if err != nil || !changed {
		t.Fatal("cleanup failed", err)
	}
	if migrated.Exists("phabricator.profiles.wikimedia") || migrated.Exists("phabricator.default_profile") {
		t.Fatal("empty logout marker/default retained")
	}
	if migrated.String("phabricator.profiles.retained.key") != "private-fixture" {
		t.Fatal("cleanup discarded a credential")
	}
}
