package config

import (
	"crypto/sha256"
	"fmt"
	"reflect"
	"strings"

	"github.com/knadh/koanf"
)

// Renaming config metadata never retrieves or relocates a provider's secret.
func migrateCredentialKey(disk *koanf.Koanf, old, destination string) (bool, error) {
	oldRef, newRef := credentialReferenceKey(old), credentialReferenceKey(destination)
	if !disk.Exists(oldRef) {
		return false, nil
	}
	if disk.Exists(newRef) {
		return false, fmt.Errorf("profile migration found two credential references for %s; configuration was not changed", destination)
	}
	var ref credentialReference
	if err := disk.Unmarshal(oldRef, &ref); err != nil {
		return false, fmt.Errorf("invalid credential reference during profile migration")
	}
	expectedOld := fmt.Sprintf("%s:%x", ref.Identity, sha256.Sum256([]byte(old)))
	if ref.Account != expectedOld {
		return false, fmt.Errorf("credential identity mismatch during profile migration; configuration was not changed")
	}
	raw, ok := disk.Get(oldRef).(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("invalid credential reference during profile migration")
	}
	raw["account"] = fmt.Sprintf("%s:%x", ref.Identity, sha256.Sum256([]byte(destination)))
	if ref.Provider == "" || ref.Provider == "keyring" || ref.Provider == "windows-credential-manager" {
		actual := ref.StorageAccount
		if actual == "" {
			actual = ref.Account
		}
		raw["storage_account"] = actual
	}
	if err := disk.Set(newRef, raw); err != nil {
		return false, err
	}
	disk.Delete(oldRef)
	return true, nil
}

// TODO(early 2027): remove the legacy flat Gerrit configuration migration.
func migrateGerritProfiles(source *koanf.Koanf) (*koanf.Koanf, bool, error) {
	disk := source.Copy()
	oldKey := "gerrit.password"
	if !disk.Exists("gerrit.username") && !disk.Exists(oldKey) && !disk.Exists(credentialReferenceKey(oldKey)) {
		return disk, false, nil
	}
	username := strings.TrimSpace(disk.String("gerrit.username"))
	if username == "" {
		username = disk.String(credentialReferenceKey(oldKey) + ".identity")
	}
	if username == "" {
		if disk.String(oldKey) == "" && !disk.Exists(credentialReferenceKey(oldKey)) {
			disk.Delete("gerrit.username")
			disk.Delete(oldKey)
			return disk, true, nil
		}
		return nil, false, fmt.Errorf("Gerrit profile migration needs a username; configuration was not changed")
	}
	name := strings.ToLower(username)
	for _, c := range name {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			name = "default"
			break
		}
	}
	prefix := "gerrit.profiles." + name + "."
	for _, field := range []string{"username", "password"} {
		old := "gerrit." + field
		if !disk.Exists(old) {
			continue
		}
		if disk.Exists(prefix+field) && !reflect.DeepEqual(disk.Get(old), disk.Get(prefix+field)) {
			return nil, false, fmt.Errorf("Gerrit profile migration conflicts with profile %q; configuration was not changed", name)
		}
		if err := disk.Set(prefix+field, disk.Get(old)); err != nil {
			return nil, false, err
		}
		disk.Delete(old)
	}
	if !disk.Exists(prefix + "username") {
		if err := disk.Set(prefix+"username", username); err != nil {
			return nil, false, err
		}
	}
	if _, err := migrateCredentialKey(disk, oldKey, prefix+"password"); err != nil {
		return nil, false, err
	}
	if disk.String("gerrit.default_profile") == "" {
		if err := disk.Set("gerrit.default_profile", name); err != nil {
			return nil, false, err
		}
	}
	return disk, true, nil
}
