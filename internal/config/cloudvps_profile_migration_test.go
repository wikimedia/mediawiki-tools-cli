package config

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/knadh/koanf"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

func TestCloudVPSMigrationPreservesCredentialProviders(t *testing.T) {
	for _, provider := range []string{"plaintext", "keyring", secrets.WindowsProvider, "keepassxc-browser"} {
		t.Run(provider, func(t *testing.T) {
			store := isolatedConfig(t)
			previousWindows := windowsCredentialStore
			windowsCredentialStore = func() (secrets.Store, error) { return store, nil }
			t.Cleanup(func() { windowsCredentialStore = previousWindows })
			source := koanf.New(".")
			old := "cloud_vps.projects.demo.credential."
			_ = source.Set(old+"id", "credential-id")
			_ = source.Set("cloud_vps.default_project", "demo")
			_ = source.Set("cloud_vps.ssh.user", "developer")
			cred := Credential{Key: old + "secret", Service: "cloudvps", Endpoint: secrets.CloudEndpoint, Account: "demo:credential-id"}
			ref, err := cred.reference()
			if err != nil {
				t.Fatal(err)
			}
			value := "private-cloud-migration-fixture"
			switch provider {
			case "plaintext":
				_ = source.Set(old+"secret", value)
			default:
				metadata := map[string]any{"provider": provider, "service": ref.Service, "endpoint": ref.Endpoint, "identity": ref.Identity, "account": ref.Account}
				if provider == "keepassxc-browser" {
					metadata["browser"] = map[string]any{"database_hash": strings.Repeat("a", 64), "entry_id": strings.Repeat("b", 32), "login": "demo:credential-id"}
					previous := readKeePassXCBrowser
					readKeePassXCBrowser = func(_ context.Context, got secrets.KeePassXCBrowser) (string, error) {
						if got.EntryID != strings.Repeat("b", 32) {
							t.Fatal("entry changed")
						}
						return value, nil
					}
					t.Cleanup(func() { readKeePassXCBrowser = previous })
				} else {
					store.values[ref.Service+ref.Account] = value
				}
				_ = source.Set(credentialReferenceKey(old+"secret"), metadata)
			}
			before := source.Raw()
			migrated, changed, err := migrateCloudVPSProfiles(source)
			if err != nil || !changed {
				t.Fatalf("migration: %v", err)
			}
			if !reflect.DeepEqual(before, source.Raw()) || store.reads != 0 {
				t.Fatal("migration mutated source or read provider")
			}
			if migrated.Exists(old+"id") || migrated.Exists(credentialReferenceKey(old+"secret")) {
				t.Fatal("legacy credential retained")
			}
			if migrated.String("cloud_vps.default_profile") != "demo" || migrated.String("cloud_vps.ssh.user") != "developer" || migrated.String("cloud_vps.default_project") != "demo" {
				t.Fatal("defaults lost")
			}
			again, changed, err := migrateCloudVPSProfiles(migrated)
			if err != nil || changed || !reflect.DeepEqual(migrated.Raw(), again.Raw()) {
				t.Fatal("not idempotent", err)
			}
			if err := PutDiskConfig(migrated); err != nil {
				t.Fatal(err)
			}
			current, err := CredentialForKey("cloud_vps.profiles.demo.secret")
			if err != nil {
				t.Fatal(err)
			}
			if got, err := current.Read(); err != nil || got != value {
				t.Fatal("migrated secret unavailable", err)
			}
			if provider == "keyring" || provider == secrets.WindowsProvider {
				if err := current.SaveWithProvider("updated-fixture", false, nil, provider); err != nil {
					t.Fatal(err)
				}
				if store.values[ref.Service+ref.Account] != "updated-fixture" || len(store.values) != 1 {
					t.Fatal("save moved physical slot")
				}
				if err := current.Delete(); err != nil {
					t.Fatal(err)
				}
				if len(store.values) != 0 {
					t.Fatal("removal abandoned original native slot")
				}
			}
		})
	}
}

func TestCloudVPSMigrationConflictsAreAtomic(t *testing.T) {
	isolatedConfig(t)
	source := koanf.New(".")
	_ = source.Set("cloud_vps.projects.demo.credential.id", "old-id")
	_ = source.Set("cloud_vps.projects.demo.credential.secret", "old-fixture")
	_ = source.Set("cloud_vps.profiles.demo.id", "new-id")
	before := source.Raw()
	if _, _, err := migrateCloudVPSProfiles(source); err == nil {
		t.Fatal("conflicting profile overwritten")
	}
	if !reflect.DeepEqual(before, source.Raw()) {
		t.Fatal("source changed on conflict")
	}
}

func TestCloudVPSProfileMigrationOnLoad(t *testing.T) {
	isolatedConfig(t)
	source := koanf.New(".")
	_ = source.Set("cloud_vps.projects.demo.credential.id", "fixture-id")
	_ = source.Set("cloud_vps.projects.demo.credential.secret", "fixture-value")
	if err := PutDiskConfig(source); err != nil {
		t.Fatal(err)
	}
	kLoaded = false
	state := State()
	if state.OnDiskKoanf.String("cloud_vps.profiles.demo.project") != "demo" || state.OnDiskKoanf.Exists("cloud_vps.projects.demo.credential") {
		t.Fatal("startup did not migrate")
	}
}
