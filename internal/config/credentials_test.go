package config

import (
	"context"
	"errors"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/adrg/xdg"
	"github.com/knadh/koanf"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

type memorySecrets struct {
	values                       map[string]string
	failSet, failGet, failDelete bool
	reads                        int
}

func (s *memorySecrets) Get(service, account string) (string, error) {
	s.reads++
	if s.failGet {
		return "", errors.New("locked")
	}
	return s.values[service+account], nil
}

func (s *memorySecrets) Set(service, account, value string) error {
	if s.failSet {
		return errors.New("locked")
	}
	s.values[service+account] = value
	return nil
}

func (s *memorySecrets) Delete(service, account string) error {
	if s.failDelete {
		return errors.New("locked")
	}
	delete(s.values, service+account)
	return nil
}

func isolatedConfig(t *testing.T) *memorySecrets {
	t.Helper()
	oldK, oldDisk, oldC, oldCDisk, oldLoaded, oldHome, oldStore := k, kOnDisk, c, cOnDisk, kLoaded, xdg.ConfigHome, credentialStore
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
	xdg.ConfigHome = t.TempDir()
	k, kOnDisk, c, cOnDisk, kLoaded = koanf.New("."), koanf.New("."), Config{}, Config{}, false
	store := &memorySecrets{values: map[string]string{}}
	credentialStore = store
	t.Cleanup(func() {
		k, kOnDisk, c, cOnDisk, kLoaded, xdg.ConfigHome, credentialStore = oldK, oldDisk, oldC, oldCDisk, oldLoaded, oldHome, oldStore
	})
	State()
	return store
}

func seedCredential(t *testing.T) Credential {
	t.Helper()
	if err := PutKeyValueOnDisk("gerrit.username", "fixture"); err != nil {
		t.Fatal(err)
	}
	if err := PutKeyValueOnDisk("gerrit.password", "legacy-secret"); err != nil {
		t.Fatal(err)
	}
	cred, err := CredentialForKey("gerrit.password")
	if err != nil {
		t.Fatal(err)
	}
	return cred
}

func TestCredentialMigrationAndDeletion(t *testing.T) {
	store := isolatedConfig(t)
	cred := seedCredential(t)
	if got, err := cred.Read(); got != "legacy-secret" || err != nil {
		t.Fatalf("legacy read: %q %v", got, err)
	}
	if store.reads != 0 {
		t.Fatal("legacy config unexpectedly opened keyring")
	}
	if err := cred.Save("vault-secret", false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(Path())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "vault-secret") || strings.Contains(string(data), "legacy-secret") {
		t.Fatal("plaintext remains on disk")
	}
	if State().Effective.Gerrit.Password != "" {
		t.Fatal("plaintext remains in effective config")
	}
	if got, err := cred.Read(); got != "vault-secret" || err != nil {
		t.Fatalf("keyring read: %q %v", got, err)
	}
	for _, changed := range []Credential{
		{Key: cred.Key, Service: cred.Service, Endpoint: "https://example.org", Account: cred.Account},
		{Key: cred.Key, Service: cred.Service, Endpoint: cred.Endpoint, Account: "another-user"},
	} {
		before := store.reads
		if _, err := changed.Read(); err == nil {
			t.Fatal("accepted mismatched credential destination")
		}
		if before != store.reads {
			t.Fatal("read mismatched keyring entry")
		}
	}
	store.failDelete = true
	if err := cred.Delete(); err == nil {
		t.Fatal("ignored keyring deletion error")
	}
	if !HasCredential(cred.Key) {
		t.Fatal("lost reference after failed deletion")
	}
	store.failDelete = false
	if err := cred.Delete(); err != nil {
		t.Fatal(err)
	}
	if HasCredential(cred.Key) || len(store.values) != 0 {
		t.Fatal("logout left credentials behind")
	}
}

func TestMigrationPreservesPlaintextOnFailure(t *testing.T) {
	for _, failure := range []string{"write", "read", "config"} {
		t.Run(failure, func(t *testing.T) {
			store := isolatedConfig(t)
			cred := seedCredential(t)
			store.failSet = failure == "write"
			store.failGet = failure == "read"
			if failure == "config" {
				if err := os.Remove(Path()); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(Path(), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			if err := cred.Save("replacement", false); err == nil {
				t.Fatal("migration unexpectedly succeeded")
			}
			if got := State().OnDiskKoanf.String(cred.Key); got != "legacy-secret" {
				t.Fatalf("lost legacy value: %q", got)
			}
			if failure != "config" {
				data, _ := os.ReadFile(Path())
				if !strings.Contains(string(data), "legacy-secret") {
					t.Fatal("lost on-disk legacy value")
				}
			}
		})
	}
}

func TestRedactionDoesNotMutateConfig(t *testing.T) {
	isolatedConfig(t)
	disk := State().OnDiskKoanf.Copy()
	for _, key := range []string{"gerrit.password", "cloud_vps.projects.demo.credential.secret", "phabricator.sites.demo.key", "service.api_token"} {
		if err := disk.Set(key, "test-secret"); err != nil {
			t.Fatal(err)
		}
	}
	redacted := Redacted(disk)
	if strings.Contains(PrettyPrint(redacted), "test-secret") {
		t.Fatal("secret visible in redacted output")
	}
	if disk.String("gerrit.password") != "test-secret" {
		t.Fatal("redaction changed stored credential")
	}
}

func TestConfigWriteRepairsPermissionsAndRejectsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permissions and symlinks")
	}
	isolatedConfig(t)
	if err := os.Chmod(os.Getenv("CI_PROJECT_DIR"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(Path(), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writePrivateConfig([]byte("{}")); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(Path())
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("config mode %v", info.Mode())
	}
	dirInfo, _ := os.Stat(os.Getenv("CI_PROJECT_DIR"))
	if dirInfo.Mode().Perm() != 0o755 {
		t.Fatalf("unexpected checkout permissions %v", dirInfo.Mode())
	}
	target := t.TempDir() + "/target"
	if err := os.WriteFile(target, []byte("untouched"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(Path()); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, Path()); err != nil {
		t.Fatal(err)
	}
	if err := writePrivateConfig([]byte("overwritten")); err == nil {
		t.Fatal("followed config symlink")
	}
	data, _ := os.ReadFile(target)
	if string(data) != "untouched" {
		t.Fatal("overwrote symlink target")
	}
}

func TestUserConfigDirectoryPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix directory permissions")
	}
	isolatedConfig(t)
	if err := os.Unsetenv("GITLAB_CI"); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(xdg.ConfigHome+"/mwcli", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writePrivateConfig([]byte("{}")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(xdg.ConfigHome + "/mwcli")
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("directory mode %v", info.Mode())
	}
}

func TestCredentialReplacementDoesNotOrphanPreviousAccount(t *testing.T) {
	store := isolatedConfig(t)
	cred := seedCredential(t)
	if err := cred.Save("vault-secret", false); err != nil {
		t.Fatal(err)
	}
	changed := cred
	changed.Account = "other-user"
	if err := changed.Save("other-secret", false); err == nil {
		t.Fatal("replaced identity without logout")
	}
	if len(store.values) != 1 {
		t.Fatal("orphaned keyring credentials")
	}
	if got, err := cred.Read(); got != "vault-secret" || err != nil {
		t.Fatalf("existing credential changed: %v", err)
	}
}

func TestKeyringReferenceDoesNotFallBackToPlaintext(t *testing.T) {
	store := isolatedConfig(t)
	cred := seedCredential(t)
	if err := cred.Save("vault-secret", false); err != nil {
		t.Fatal(err)
	}
	if err := PutKeyValueOnDisk(cred.Key, "leftover"); err != nil {
		t.Fatal(err)
	}
	store.failGet = true
	if value, err := cred.Read(); err == nil || value != "" {
		t.Fatal("locked keyring fell back to plaintext")
	}
}

func TestGUIProviderSwitchPreservesIdentityAndNeverUnlocksDatabaseFile(t *testing.T) {
	for _, failure := range []string{"", "denied", "wrong-account", "mismatch"} {
		t.Run(failure, func(t *testing.T) {
			isolatedConfig(t)
			cred := seedCredential(t)
			oldFind, oldGUI := findKeePassXCBrowser, readKeePassXCBrowser
			t.Cleanup(func() { findKeePassXCBrowser, readKeePassXCBrowser = oldFind, oldGUI })
			seedLegacyKeePassXCLink(t, cred)
			if failure == "mismatch" {
				if err := PutKeyValueOnDisk(cred.Key, "different-plaintext"); err != nil {
					t.Fatal(err)
				}
			}
			calls := 0
			findKeePassXCBrowser = func(_ context.Context, path string) (secrets.KeePassXCBrowser, string, error) {
				calls++
				if path != "Custom/Gerrit" {
					t.Fatal("lost existing entry path")
				}
				ref := secrets.KeePassXCBrowser{DatabaseHash: strings.Repeat("a", 64), EntryID: strings.Repeat("b", 32), Login: "fixture"}
				if failure == "denied" {
					return ref, "", errors.New("denied")
				}
				if failure == "wrong-account" {
					ref.Login = "someone-else"
				}
				return ref, "legacy-secret", nil
			}
			before, err := os.ReadFile(Path())
			if err != nil {
				t.Fatal(err)
			}
			err = cred.UseKeePassXC(context.Background(), "")
			if calls != 1 || (err != nil) != (failure != "") {
				t.Fatalf("unexpected GUI switch result: %v", err)
			}
			if failure != "" {
				after, err := os.ReadFile(Path())
				if err != nil {
					t.Fatal(err)
				}
				if string(after) != string(before) {
					t.Fatal("failed GUI switch changed config")
				}
				return
			}
			data, err := os.ReadFile(Path())
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(data), "legacy-secret") || strings.Contains(string(data), "Passwords.kdbx") {
				t.Fatal("GUI reference retained direct-access data")
			}
			readKeePassXCBrowser = func(secrets.KeePassXCBrowser) (string, error) { return "gui-secret", nil }
			if value, err := cred.Read(); err != nil || value != "gui-secret" {
				t.Fatalf("GUI read failed: %v", err)
			}
			changed := cred
			changed.Account = "someone-else"
			if _, err := changed.Read(); err == nil {
				t.Fatal("used GUI entry for another account")
			}
			readKeePassXCBrowser = func(secrets.KeePassXCBrowser) (string, error) { return "", errors.New("locked") }
			if err := PutKeyValueOnDisk(cred.Key, "stale-plaintext"); err != nil {
				t.Fatal(err)
			}
			if value, err := cred.Read(); err == nil || value != "" {
				t.Fatal("locked GUI fell back to plaintext")
			}
			if err := cred.Delete(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestGUIProviderMigrationRemovesPlaintextOnlyAfterVerification(t *testing.T) {
	isolatedConfig(t)
	cred := seedCredential(t)
	old := findKeePassXCBrowser
	t.Cleanup(func() { findKeePassXCBrowser = old })
	findKeePassXCBrowser = func(_ context.Context, path string) (secrets.KeePassXCBrowser, string, error) {
		if path != "wm/gerrit.password" {
			t.Fatal("unexpected default path")
		}
		return secrets.KeePassXCBrowser{DatabaseHash: strings.Repeat("a", 64), EntryID: strings.Repeat("b", 32), Login: "fixture"}, "legacy-secret", nil
	}
	if err := cred.UseKeePassXC(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	if State().OnDiskKoanf.Exists(cred.Key) {
		t.Fatal("plaintext survived GUI migration")
	}
}

func seedLegacyKeePassXCLink(t *testing.T, cred Credential) {
	t.Helper()
	ref, err := cred.reference()
	if err != nil {
		t.Fatal(err)
	}
	disk := State().OnDiskKoanf.Copy()
	disk.Delete(cred.Key)
	if err := disk.Set(credentialReferenceKey(cred.Key), map[string]any{"service": ref.Service, "endpoint": ref.Endpoint, "account": ref.Account, "identity": ref.Identity, "provider": "keepassxc", "keepassxc": map[string]any{"entry": "Custom/Gerrit", "database": "Passwords.kdbx", "executable": "never-run-this"}}); err != nil {
		t.Fatal(err)
	}
	if err := PutDiskConfig(disk); err != nil {
		t.Fatal(err)
	}
	if _, err := cred.Read(); err == nil || !strings.Contains(err.Error(), "config secrets use-keepassxc") {
		t.Fatal("old link did not request GUI relinking")
	}
}

func TestKeePassXCRelinkVerifiesBeforeRemovingLeftoverPlaintext(t *testing.T) {
	for _, mode := range []string{"success", "locked", "mismatch"} {
		t.Run(mode, func(t *testing.T) {
			isolatedConfig(t)
			cred := seedCredential(t)
			oldFind, oldRead := findKeePassXCBrowser, readKeePassXCBrowser
			t.Cleanup(func() { findKeePassXCBrowser, readKeePassXCBrowser = oldFind, oldRead })
			findKeePassXCBrowser = func(context.Context, string) (secrets.KeePassXCBrowser, string, error) {
				return secrets.KeePassXCBrowser{DatabaseHash: strings.Repeat("a", 64), EntryID: strings.Repeat("b", 32), Login: "fixture"}, "legacy-secret", nil
			}
			if err := cred.UseKeePassXC(context.Background(), ""); err != nil {
				t.Fatal(err)
			}
			if err := PutKeyValueOnDisk(cred.Key, "legacy-secret"); err != nil {
				t.Fatal(err)
			}
			calls := 0
			readKeePassXCBrowser = func(secrets.KeePassXCBrowser) (string, error) {
				calls++
				if mode == "locked" {
					return "", errors.New("locked")
				}
				if mode == "mismatch" {
					return "different", nil
				}
				return "legacy-secret", nil
			}
			err := cred.UseKeePassXC(context.Background(), "")
			if calls != 1 || (err != nil) != (mode != "success") {
				t.Fatalf("unexpected verification result: %v", err)
			}
			if mode != "success" {
				if State().OnDiskKoanf.String(cred.Key) != "legacy-secret" {
					t.Fatal("failed verification removed plaintext")
				}
				return
			}
			if err := PutKeyValueOnDisk("timer_last_emitted_event", "fixture"); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(Path())
			if err != nil {
				t.Fatal(err)
			}
			if State().EffectiveKoanf.String(cred.Key) != "" || strings.Contains(string(data), "legacy-secret") {
				t.Fatal("later config write restored plaintext")
			}
		})
	}
}
