package wiki

import (
	"fmt"
	"sort"
	"strings"

	"gitlab.wikimedia.org/repos/releng/cli/internal/authprofile"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiauth"
)

type wikiProfiles struct{}

var _ authprofile.Backend = wikiProfiles{}

func wikiDefaultRealm() string {
	realm, _, _ := strings.Cut(config.State().OnDiskKoanf.String("wiki.default_profile"), "/")
	return realm
}

func splitWikiProfile(selector string) (realm, name string, err error) {
	realm, name = wikiRealm, selector
	if strings.Contains(name, "/") {
		qualifiedRealm, qualifiedName, _ := strings.Cut(name, "/")
		if realm != "" && realm != qualifiedRealm {
			return "", "", fmt.Errorf("--realm and --profile disagree")
		}
		realm, name = qualifiedRealm, qualifiedName
	}
	return realm, name, nil
}

func selectedWikiProfile(selector string) (config.WikiProfile, error) {
	realm, name, err := splitWikiProfile(selector)
	if err != nil {
		return config.WikiProfile{}, err
	}
	if realm == "" && wiki != "" {
		endpoint, err := wikiauth.Endpoint(wiki)
		if err != nil {
			return config.WikiProfile{}, err
		}
		realm, err = realmForWiki(endpoint)
		if err != nil {
			return config.WikiProfile{}, err
		}
	}
	if realm == "" {
		globalRealm, globalName, _ := strings.Cut(config.State().OnDiskKoanf.String("wiki.default_profile"), "/")
		realm = globalRealm
		if name == "" {
			name = globalName
		}
	}
	if realm == "" && name != "" {
		realm = "wikimedia"
	}
	if name == "" {
		name = config.State().OnDiskKoanf.String("wiki.realms." + realm + ".default_profile")
	}
	return config.LoadWikiProfile(realm, name)
}

func (wikiProfiles) Selected(selector string) (string, error) {
	a, err := selectedWikiProfile(selector)
	if err != nil {
		return "", err
	}
	return a.Realm + "/" + a.Name, nil
}

func (wikiProfiles) Profiles() ([]authprofile.Profile, error) {
	var profiles []authprofile.Profile
	global := config.State().OnDiskKoanf.String("wiki.default_profile")
	for _, a := range completionProfiles() {
		name := a.Realm + "/" + a.Name
		details := a.Method
		if len(a.Sites) > 0 {
			details += fmt.Sprintf("; %d sites", len(a.Sites))
		} else if a.Realm == "wikimedia" {
			details += "; Wikimedia sites"
		}
		if name != global && config.State().OnDiskKoanf.String("wiki.realms."+a.Realm+".default_profile") == a.Name {
			details += "; realm default"
		}
		profiles = append(profiles, authprofile.Profile{Name: name, Username: a.Identity, Endpoint: a.Wiki, Details: details, Default: name == global})
	}
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].Name < profiles[j].Name })
	return profiles, nil
}

func (wikiProfiles) SetDefault(selector string) error {
	a, err := selectedWikiProfile(selector)
	if err != nil {
		return err
	}
	disk := config.State().OnDiskKoanf.Copy()
	if err := disk.Set("wiki.realms."+a.Realm+".default_profile", a.Name); err != nil {
		return err
	}
	if err := disk.Set("wiki.default_profile", a.Realm+"/"+a.Name); err != nil {
		return err
	}
	return config.PutDiskConfig(disk)
}

func (wikiProfiles) Remove(selector string) (bool, error) {
	a, err := selectedWikiProfile(selector)
	if err != nil {
		return false, err
	}
	if err := a.Credential().Delete(); err != nil {
		return false, err
	}
	prefix, _ := config.WikiProfilePrefix(a.Realm, a.Name)
	disk := config.State().OnDiskKoanf.Copy()
	disk.Delete(strings.TrimSuffix(prefix, "."))
	removedDefault := false
	key := "wiki.realms." + a.Realm + ".default_profile"
	if disk.String(key) == a.Name {
		disk.Delete(key)
		removedDefault = true
	}
	if disk.String("wiki.default_profile") == a.Realm+"/"+a.Name {
		disk.Delete("wiki.default_profile")
		removedDefault = true
	}
	return removedDefault, config.PutDiskConfig(disk)
}

func checkWikiProfileReplacement(realm, name, method string, replace bool) (*config.WikiProfile, error) {
	prefix, err := config.WikiProfilePrefix(realm, name)
	if err != nil {
		return nil, err
	}
	if !config.State().OnDiskKoanf.Exists(strings.TrimSuffix(prefix, ".")) {
		return nil, nil
	}
	if !replace {
		return nil, fmt.Errorf("profile already exists; use --replace to reauthorize the saved profile")
	}
	previous, err := config.LoadWikiProfile(realm, name)
	if err != nil {
		return nil, err
	}
	if previous.Method != method {
		return nil, fmt.Errorf("--replace cannot change the authentication method; use a separate profile")
	}
	return &previous, nil
}

func normalizeProfileSites(sites []string, endpoint string) ([]string, error) {
	values := map[string]bool{endpoint: true}
	for _, site := range sites {
		normalized, err := wikiauth.Endpoint(site)
		if err != nil {
			return nil, err
		}
		values[normalized] = true
	}
	result := make([]string, 0, len(values))
	for site := range values {
		result = append(result, site)
	}
	sort.Strings(result)
	return result, nil
}
