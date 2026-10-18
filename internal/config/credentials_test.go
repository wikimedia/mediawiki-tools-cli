package config

import (
	"errors"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/adrg/xdg"
	"github.com/knadh/koanf"
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
