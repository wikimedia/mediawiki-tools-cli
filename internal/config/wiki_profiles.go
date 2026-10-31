package config

import (
	"fmt"
	"strings"

	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiauth"
)

type WikiProfile struct {
	Realm, Name    string
	Identity       string   `koanf:"identity"`
	Sites          []string `koanf:"sites"`
	Username       string   `koanf:"username"`
	Method         string   `koanf:"method"`
	Endpoint       string   `koanf:"endpoint"`
	Wiki           string   `koanf:"wiki"`
	ClientID       string   `koanf:"client_id"`
	RedirectURI    string   `koanf:"redirect_uri"`
	RequestedScope string   `koanf:"requested_scope"`
}

func WikiProfilePrefix(realm, name string) (string, error) {
	if !wikiauth.ValidName(realm) || !wikiauth.ValidName(name) {
		return "", fmt.Errorf("realm and profile names must contain only letters, digits, underscores or hyphens")
	}
	return "wiki.profiles." + realm + "." + name + ".", nil
}

func LoadWikiProfile(realm, name string) (WikiProfile, error) {
	prefix, err := WikiProfilePrefix(realm, name)
	if err != nil {
		return WikiProfile{}, err
	}
	a := WikiProfile{Realm: realm, Name: name}
	if err := State().OnDiskKoanf.Unmarshal(strings.TrimSuffix(prefix, "."), &a); err != nil {
		return a, fmt.Errorf("invalid saved wiki profile")
	}
	// Identity cannot be overridden by fields inside the profile.
	a.Realm, a.Name = realm, name
	if a.Method == "bot-password" {
		a.Method = "password"
	}
	if a.Identity == "" {
		a.Identity = a.Username
		if a.Method == "password" {
			a.Identity = strings.SplitN(a.Username, "@", 2)[0]
		}
	}
	if a.Username == "" || a.Endpoint == "" || a.Wiki == "" || (a.Method != "oauth" && a.Method != "password") {
		return a, fmt.Errorf("wiki profile %s/%s is not configured; use wiki auth login", realm, name)
	}
	if realm == "wikimedia" && a.Endpoint != wikiauth.WikimediaAuthority {
		return a, fmt.Errorf("invalid Wikimedia credential authority")
	}
	if a.Method == "oauth" && (realm != "wikimedia" || a.ClientID == "") {
		return a, fmt.Errorf("OAuth is currently supported only for Wikimedia accounts")
	}
	for _, site := range a.Sites {
		normalized, err := wikiauth.Endpoint(site)
		if err != nil || normalized != site {
			return a, fmt.Errorf("invalid wiki profile site scope")
		}
		if realm == "wikimedia" && !wikiauth.IsWikimedia(site) {
			return a, fmt.Errorf("Wikimedia profile includes a non-Wikimedia site")
		}
	}
	if !a.Allows(a.Wiki) {
		return a, fmt.Errorf("saved wiki is outside the profile's site scope")
	}
	return a, nil
}

func (a WikiProfile) Credential() Credential {
	prefix, err := WikiProfilePrefix(a.Realm, a.Name)
	if err != nil {
		return Credential{Service: "wiki", Endpoint: a.Endpoint, Account: a.Username}
	}
	leaf := "password"
	if a.Method == "oauth" {
		leaf = "token"
	}
	return Credential{Key: prefix + leaf, Service: "wiki", Endpoint: a.Endpoint, Account: a.Username}
}

func (a WikiProfile) Metadata() map[string]string {
	prefix, err := WikiProfilePrefix(a.Realm, a.Name)
	if err != nil {
		return nil
	}
	return map[string]string{prefix + "username": a.Username, prefix + "identity": a.Identity, prefix + "method": a.Method, prefix + "endpoint": a.Endpoint, prefix + "wiki": a.Wiki, prefix + "client_id": a.ClientID, prefix + "redirect_uri": a.RedirectURI, prefix + "requested_scope": a.RequestedScope}
}

func WikiRealmEndpoints(realm string) []string {
	endpoints := State().OnDiskKoanf.Strings("wiki.realms." + realm + ".endpoints")
	if endpoint := State().OnDiskKoanf.String("wiki.realms." + realm + ".endpoint"); endpoint != "" {
		endpoints = append(endpoints, endpoint)
	}
	return endpoints
}

func WikiRealmAllows(realm, endpoint string) bool {
	return wikiauth.Allows(realm, endpoint, WikiRealmEndpoints(realm))
}

func (a WikiProfile) Allows(endpoint string) bool {
	if len(a.Sites) > 0 {
		for _, site := range a.Sites {
			if site == endpoint {
				return true
			}
		}
		return false
	}
	return WikiRealmAllows(a.Realm, endpoint)
}
