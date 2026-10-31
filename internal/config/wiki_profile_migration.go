package config

import (
	"fmt"
	"reflect"

	"github.com/knadh/koanf"
)

// TODO(early 2027): remove the wiki account/default_realm migration and hidden
// --account / --user aliases after the profile-based CLI has shipped.
func migrateWikiProfiles(source *koanf.Koanf) (*koanf.Koanf, bool, error) {
	disk := source.Copy()
	changed := false
	for _, realm := range disk.MapKeys("wiki.accounts") {
		for _, name := range disk.MapKeys("wiki.accounts." + realm) {
			old, destination := "wiki.accounts."+realm+"."+name, "wiki.profiles."+realm+"."+name
			if disk.Exists(destination) && !reflect.DeepEqual(disk.Get(old), disk.Get(destination)) {
				return nil, false, fmt.Errorf("wiki profile migration conflicts at %s; configuration was not changed", destination)
			}
			if err := disk.Set(destination, disk.Get(old)); err != nil {
				return nil, false, err
			}
			for _, field := range []string{"password", "token"} {
				if _, err := migrateCredentialKey(disk, old+"."+field, destination+"."+field); err != nil {
					return nil, false, err
				}
			}
			disk.Delete(old)
			changed = true
		}
	}
	if disk.Exists("wiki.accounts") {
		disk.Delete("wiki.accounts")
		changed = true
	}
	for _, realm := range disk.MapKeys("wiki.realms") {
		old := "wiki.realms." + realm + ".default_account"
		if disk.Exists(old) {
			destination := "wiki.realms." + realm + ".default_profile"
			if disk.String(destination) == "" {
				if err := disk.Set(destination, disk.Get(old)); err != nil {
					return nil, false, err
				}
			}
			disk.Delete(old)
			changed = true
		}
	}
	if disk.Exists("wiki.default_realm") {
		realm := disk.String("wiki.default_realm")
		name := disk.String("wiki.realms." + realm + ".default_profile")
		if disk.String("wiki.default_profile") == "" && realm != "" && name != "" {
			if err := disk.Set("wiki.default_profile", realm+"/"+name); err != nil {
				return nil, false, err
			}
		}
		disk.Delete("wiki.default_realm")
		changed = true
	}
	for _, realm := range disk.MapKeys("wiki.profiles") {
		for _, name := range disk.MapKeys("wiki.profiles." + realm) {
			key := "wiki.profiles." + realm + "." + name + ".method"
			if disk.String(key) == "bot-password" {
				if err := disk.Set(key, "password"); err != nil {
					return nil, false, err
				}
				changed = true
			}
		}
	}
	return disk, changed, nil
}
