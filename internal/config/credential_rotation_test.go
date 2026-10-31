package config

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

func cloudRotationCredential(id string) Credential {
	return Credential{Key: "cloud_vps.profiles.fixture.secret", Service: "cloudvps", Endpoint: secrets.CloudEndpoint, Account: "fixture:" + id}
}

func cloudRotationMetadata(id string) map[string]string {
	return map[string]string{"cloud_vps.profiles.fixture.project": "fixture", "cloud_vps.profiles.fixture.id": id, "cloud_vps.default_profile": "fixture"}
}

func mockRotationKeePassXC(t *testing.T, denyCleanup bool) map[string]string {
	t.Helper()
	oldWrite, oldRead, oldDelete := writeKeePassXCBrowser, readKeePassXCBrowser, deleteRetainedKeePassXC
	t.Cleanup(func() {
		writeKeePassXCBrowser, readKeePassXCBrowser, deleteRetainedKeePassXC = oldWrite, oldRead, oldDelete
	})
	entries := map[string]string{}
	writeKeePassXCBrowser = func(_ context.Context, existing *secrets.KeePassXCBrowser, _, account, _, value string) (secrets.KeePassXCBrowser, error) {
		id := strings.Repeat("a", 32)
		if account == "fixture:new-id" {
			id = strings.Repeat("b", 32)
		}
		if existing != nil && existing.Login != account {
			t.Fatal("rotation attempted to mutate another account's entry")
		}
		entries[id] = value
		return secrets.KeePassXCBrowser{DatabaseHash: strings.Repeat("c", 64), EntryID: id, Login: account}, nil
	}
	readKeePassXCBrowser = func(ref secrets.KeePassXCBrowser) (string, error) {
		value, ok := entries[ref.EntryID]
		if !ok {
			return "", secrets.ErrNotFound
		}
		return value, nil
	}
	deleteRetainedKeePassXC = func(_ context.Context, ref secrets.KeePassXCBrowser) error {
		if State().OnDiskKoanf.String("cloud_vps.profiles.fixture.id") != "new-id" {
			t.Fatal("source deleted before replacement was committed")
		}
		if denyCleanup {
			return errors.New("fixture: cleanup denied")
		}
		delete(entries, ref.EntryID)
		return nil
	}
	return entries
}

func TestCredentialRotationAcrossProviders(t *testing.T) {
	for _, provider := range []string{"keyring", secrets.WindowsProvider, "keepassxc", "plaintext"} {
		t.Run(provider, func(t *testing.T) {
			native := isolatedConfig(t)
			windows := mockWindowsStore(t)
			gui := mockRotationKeePassXC(t, false)
			old, next := cloudRotationCredential("old-id"), cloudRotationCredential("new-id")
			if err := old.SaveWithProvider("old-private-fixture", false, cloudRotationMetadata("old-id"), provider); err != nil {
				t.Fatal(err)
			}
			if provider == "keyring" || provider == secrets.WindowsProvider {
				// Migrated profiles may pin their previous native storage account.
				ref, err := old.reference()
				if err != nil {
					t.Fatal(err)
				}
				if err := PutKeyValueOnDisk(credentialReferenceKey(old.Key)+".storage_account", ref.Account); err != nil {
					t.Fatal(err)
				}
			}
			// Rotation must retain the existing provider even if the global default differs.
			if err := PutKeyValueOnDisk("secrets.provider", "plaintext"); err != nil {
				t.Fatal(err)
			}
			if err := next.Replace(old, "new-private-fixture", false, cloudRotationMetadata("new-id"), ""); err != nil {
				t.Fatal(err)
			}
			if got, err := next.Read(); err != nil || got != "new-private-fixture" {
				t.Fatal("replacement unreadable", err)
			}
			if got, err := next.CredentialProvider(""); err != nil || got != provider {
				t.Fatal("rotation changed provider", err)
			}
			if State().OnDiskKoanf.String("cloud_vps.profiles.fixture.id") != "new-id" || State().OnDiskKoanf.String("cloud_vps.default_profile") != "fixture" {
				t.Fatal("profile metadata lost")
			}
			entries, err := DetachedSecrets("")
			if err != nil || len(entries) != 0 {
				t.Fatal("successful rotation left tracked source", err)
			}
			switch provider {
			case "keyring":
				if len(native.values) != 1 {
					t.Fatal("old keyring slot retained")
				}
			case secrets.WindowsProvider:
				if len(windows.values) != 1 {
					t.Fatal("old Windows slot retained")
				}
			case "keepassxc":
				if len(gui) != 1 {
					t.Fatal("old KeePassXC entry retained")
				}
			}
			data, err := os.ReadFile(Path())
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(data), "old-private-fixture") || provider != "plaintext" && strings.Contains(string(data), "new-private-fixture") {
				t.Fatal("rotation leaked plaintext")
			}
		})
	}
}

func TestCredentialRotationPreservesSourceOnFailure(t *testing.T) {
	for _, failure := range []string{"write", "conflict", "changed profile", "provider switch"} {
		t.Run(failure, func(t *testing.T) {
			native := isolatedConfig(t)
			old, next := cloudRotationCredential("old-id"), cloudRotationCredential("new-id")
			if err := old.SaveWithProvider("old-fixture", false, cloudRotationMetadata("old-id"), "keyring"); err != nil {
				t.Fatal(err)
			}
			provider := ""
			switch failure {
			case "write":
				native.failSet = true
			case "conflict":
				ref, err := next.reference()
				if err != nil {
					t.Fatal(err)
				}
				native.values[ref.Service+ref.Account] = "unregistered-fixture"
			case "changed profile":
				old.Account = "fixture:stale-id"
			case "provider switch":
				provider = "plaintext"
			}
			before, err := os.ReadFile(Path())
			if err != nil {
				t.Fatal(err)
			}
			if err := next.Replace(old, "new-fixture", false, cloudRotationMetadata("new-id"), provider); err == nil {
				t.Fatal("unsafe replacement succeeded")
			}
			native.failSet = false
			current := cloudRotationCredential("old-id")
			if got, err := current.Read(); err != nil || got != "old-fixture" {
				t.Fatal("failed rotation lost source", err)
			}
			after, err := os.ReadFile(Path())
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Fatal("failed rotation changed configuration")
			}
		})
	}
}

func TestCredentialRotationRetainsDeniedKeePassXCCleanup(t *testing.T) {
	isolatedConfig(t)
	gui := mockRotationKeePassXC(t, true)
	old, next := cloudRotationCredential("old-id"), cloudRotationCredential("new-id")
	if err := old.SaveWithProvider("old-fixture", false, cloudRotationMetadata("old-id"), "keepassxc"); err != nil {
		t.Fatal(err)
	}
	err := next.Replace(old, "new-fixture", false, cloudRotationMetadata("new-id"), "")
	if err == nil || !strings.Contains(err.Error(), "replacement saved; source cleanup failed") {
		t.Fatalf("partial success not reported: %v", err)
	}
	if got, err := next.Read(); err != nil || got != "new-fixture" {
		t.Fatal("cleanup failure undid replacement", err)
	}
	entries, err := DetachedSecrets("")
	if err != nil || len(entries) != 1 || len(gui) != 2 {
		t.Fatal("denied cleanup did not retain source record", err)
	}
}
