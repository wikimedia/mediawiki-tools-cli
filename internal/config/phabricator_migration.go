package config

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/knadh/koanf"
)

// TODO(early 2027): remove legacy Phabricator site/flat-config migration and
// the hidden --site alias once users have upgraded to profile-based config.
// External phab.cfg files remain user-managed and are never rewritten here.
func migratePhabricatorProfiles(source *koanf.Koanf) (*koanf.Koanf, bool, error) {
	disk := source.Copy()
	changed := false
	move := func(old, destination string) error {
		if !disk.Exists(old) {
			return nil
		}
		if disk.Exists(destination) && !reflect.DeepEqual(disk.Get(old), disk.Get(destination)) {
			return fmt.Errorf("Phabricator profile migration conflicts at %s; configuration was not changed", destination)
		}
		if err := disk.Set(destination, disk.Get(old)); err != nil {
			return err
		}
		disk.Delete(old)
		changed = true
		return nil
	}
	renameCredential := func(old, destination string) error {
		moved, err := migrateCredentialKey(disk, old, destination)
		changed = changed || moved
		return err
	}

	for _, name := range disk.MapKeys("phabricator.sites") {
		old, destination := "phabricator.sites."+name, "phabricator.profiles."+name
		if err := move(old, destination); err != nil {
			return nil, false, err
		}
		if err := renameCredential(old+".key", destination+".key"); err != nil {
			return nil, false, err
		}
	}
	if disk.Exists("phabricator.sites") {
		disk.Delete("phabricator.sites")
		changed = true
	}
	for _, key := range []string{"phabricator.default_site", "phabricator.site"} {
		if !disk.Exists(key) {
			continue
		}
		if disk.String("phabricator.default_profile") == "" {
			if err := disk.Set("phabricator.default_profile", disk.String(key)); err != nil {
				return nil, false, err
			}
		}
		disk.Delete(key)
		changed = true
	}
	// Move the original flat CLI configuration into its selected/default profile.
	fields := []string{"url", "username", "key", "default_project", "cache_path", "logged_out"}
	hasFlat := disk.Exists(credentialReferenceKey("phabricator.key"))
	for _, field := range fields {
		hasFlat = hasFlat || disk.Exists("phabricator."+field)
	}
	if hasFlat {
		name := strings.TrimSpace(disk.String("phabricator.default_profile"))
		if name == "" {
			name = "wikimedia"
		}
		for _, field := range fields {
			if err := move("phabricator."+field, "phabricator.profiles."+name+"."+field); err != nil {
				return nil, false, err
			}
		}
		if err := renameCredential("phabricator.key", "phabricator.profiles."+name+".key"); err != nil {
			return nil, false, err
		}
		if err := disk.Set("phabricator.default_profile", name); err != nil {
			return nil, false, err
		}
		changed = true
	}
	// Old logout left tombstones to suppress phab.cfg fallback. Explicit import
	// replaced that fallback, so empty tombstones no longer serve a purpose.
	for _, name := range disk.MapKeys("phabricator.profiles") {
		prefix := "phabricator.profiles." + name
		if !disk.Bool(prefix+".logged_out") || disk.String(prefix+".key") != "" || disk.Exists(credentialReferenceKey(prefix+".key")) {
			continue
		}
		disk.Delete(prefix)
		if disk.String("phabricator.default_profile") == name {
			disk.Delete("phabricator.default_profile")
		}
		changed = true
	}

	return disk, changed, nil
}

func isConfigCompletion() bool {
	return len(os.Args) > 1 && (os.Args[1] == "__complete" || os.Args[1] == "__completeNoDesc")
}
