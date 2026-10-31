package config

import (
	"fmt"
	"sort"

	"github.com/knadh/koanf"
)

// TODO(early 2027): remove the project-keyed Cloud VPS credential migration.
// Only configuration metadata moves: external values are never read or rewritten.
func migrateCloudVPSProfiles(source *koanf.Koanf) (*koanf.Koanf, bool, error) {
	disk := source.Copy()
	projects := disk.MapKeys("cloud_vps.projects")
	sort.Strings(projects)
	changed := false
	for _, project := range projects {
		old := "cloud_vps.projects." + project + ".credential."
		if !disk.Exists(old+"id") && !disk.Exists(old+"secret") && !disk.Exists(credentialReferenceKey(old+"secret")) {
			continue
		}
		prefix := "cloud_vps.profiles." + project + "."
		if disk.Exists("cloud_vps.profiles." + project) {
			return nil, false, fmt.Errorf("Cloud VPS migration conflicts with profile %q; configuration was not changed", project)
		}
		if disk.String(old+"id") == "" {
			return nil, false, fmt.Errorf("Cloud VPS migration needs an application credential ID for project %q; configuration was not changed", project)
		}
		if err := disk.Set(prefix+"project", project); err != nil {
			return nil, false, err
		}
		for _, field := range []string{"id", "secret"} {
			if disk.Exists(old + field) {
				if err := disk.Set(prefix+field, disk.Get(old+field)); err != nil {
					return nil, false, err
				}
			}
		}
		if _, err := migrateCredentialKey(disk, old+"secret", prefix+"secret"); err != nil {
			return nil, false, err
		}
		disk.Delete("cloud_vps.projects." + project + ".credential")
		changed = true
	}
	if changed && disk.String("cloud_vps.default_profile") == "" {
		name := disk.String("cloud_vps.default_project")
		if !disk.Exists("cloud_vps.profiles." + name) {
			names := disk.MapKeys("cloud_vps.profiles")
			if len(names) == 1 {
				name = names[0]
			} else {
				name = ""
			}
		}
		if name != "" {
			if err := disk.Set("cloud_vps.default_profile", name); err != nil {
				return nil, false, err
			}
		}
	}
	return disk, changed, nil
}
