package config

import (
	"sort"
	"strings"

	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

// ProfileInfo is public configuration metadata, never a credential value.
type ProfileInfo struct {
	Service      string   `json:"service"`
	Name         string   `json:"name"`
	Account      string   `json:"account,omitempty"`
	Method       string   `json:"method"`
	Default      bool     `json:"default"`
	RealmDefault bool     `json:"realm_default,omitempty"`
	Endpoint     string   `json:"endpoint,omitempty"`
	Sites        []string `json:"sites,omitempty"`
	SiteScope    string   `json:"site_scope,omitempty"`
	SecretKey    string   `json:"secret_key"`
	Provider     string   `json:"provider"`
	Status       string   `json:"status"`
}

// ProfileInventory enumerates every supported service using metadata only.
// It never opens a password manager, reads a pairing file, or calls a service.
func ProfileInventory() []ProfileInfo {
	state := State()
	disk := state.OnDiskKoanf
	effective := state.EffectiveKoanf
	profiles := make([]ProfileInfo, 0)
	appendProfile := func(p ProfileInfo) {
		key := credentialReferenceKey(p.SecretKey)
		switch {
		case disk.Exists(key):
			var reference credentialReference
			if disk.Unmarshal(key, &reference) != nil {
				p.Provider = "unknown"
				p.Status = "invalid reference"
				break
			}
			p.Provider = reference.Provider
			switch p.Provider {
			case "":
				p.Provider = "keyring"
			case "keepassxc-browser":
				p.Provider = "keepassxc"
			}
			p.Status = "linked (not checked)"
		case effective.String(p.SecretKey) != "":
			p.Provider = "plaintext"
			p.Status = "plaintext"
		default:
			p.Provider = "-"
			p.Status = "no credential"
		}
		profiles = append(profiles, p)
	}
	for _, service := range []string{"gerrit", "phabricator"} {
		for _, name := range effective.MapKeys(service + ".profiles") {
			prefix := service + ".profiles." + name + "."
			method, leaf, endpoint := "password", "password", secrets.GerritEndpoint
			if service == "phabricator" {
				method, leaf, endpoint = "api-token", "key", secrets.PhabricatorEndpoint
			}
			if custom := effective.String(prefix + "url"); custom != "" {
				endpoint = custom
			}
			appendProfile(ProfileInfo{Service: service, Name: name, Account: effective.String(prefix + "username"), Method: method, Default: effective.String(service+".default_profile") == name, Endpoint: endpoint, SecretKey: prefix + leaf})
		}
	}
	for _, realm := range disk.MapKeys("wiki.profiles") {
		for _, name := range disk.MapKeys("wiki.profiles." + realm) {
			prefix := "wiki.profiles." + realm + "." + name + "."
			method := disk.String(prefix + "method")
			leaf := "password"
			if method == "oauth" {
				leaf = "token"
			}
			sites := disk.Strings(prefix + "sites")
			scope := "explicit sites"
			if len(sites) == 0 {
				if realm == "wikimedia" {
					scope = "Wikimedia shared-login sites"
				} else {
					scope = "realm sites"
					sites = WikiRealmEndpoints(realm)
				}
			}
			qualified := realm + "/" + name
			appendProfile(ProfileInfo{Service: "wiki", Name: qualified, Account: disk.String(prefix + "username"), Method: method, Default: disk.String("wiki.default_profile") == qualified, RealmDefault: disk.String("wiki.realms."+realm+".default_profile") == name, Endpoint: disk.String(prefix + "wiki"), Sites: sites, SiteScope: scope, SecretKey: prefix + leaf})
		}
	}
	for _, name := range effective.MapKeys("cloud_vps.projects") {
		prefix := "cloud_vps.projects." + name + ".credential."
		appendProfile(ProfileInfo{Service: "cloudvps", Name: name, Account: effective.String(prefix + "id"), Method: "application-credential", Default: effective.String("cloud_vps.default_project") == name, Endpoint: secrets.CloudEndpoint, SecretKey: prefix + "secret"})
	}
	sort.Slice(profiles, func(i, j int) bool {
		if profiles[i].Service == profiles[j].Service {
			return strings.Compare(profiles[i].Name, profiles[j].Name) < 0
		}
		return profiles[i].Service < profiles[j].Service
	})
	return profiles
}
