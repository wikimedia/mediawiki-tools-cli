package config

import (
	"fmt"
	"strings"

	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiauth"
)

type WikiAccount struct {
	Realm, Name string
	Username    string `koanf:"username"`
	Method      string `koanf:"method"`
	Endpoint    string `koanf:"endpoint"`
	Wiki        string `koanf:"wiki"`
	ClientID    string `koanf:"client_id"`
	RedirectURI string `koanf:"redirect_uri"`
}

func WikiAccountPrefix(realm, name string) (string, error) {
	if !wikiauth.ValidName(realm) || !wikiauth.ValidName(name) {
		return "", fmt.Errorf("realm and account names must contain only letters, digits, underscores or hyphens")
	}
	return "wiki.accounts." + realm + "." + name + ".", nil
}

func LoadWikiAccount(realm, name string) (WikiAccount, error) {
	prefix, err := WikiAccountPrefix(realm, name)
	if err != nil {
		return WikiAccount{}, err
	}
	a := WikiAccount{Realm: realm, Name: name}
	if err := State().OnDiskKoanf.Unmarshal(strings.TrimSuffix(prefix, "."), &a); err != nil {
		return a, fmt.Errorf("invalid saved wiki account")
	}
	// Identity cannot be overridden by fields inside the profile.
	a.Realm, a.Name = realm, name
	if a.Username == "" || a.Endpoint == "" || a.Wiki == "" || (a.Method != "oauth" && a.Method != "bot-password") {
		return a, fmt.Errorf("wiki account %s/%s is not configured; use wiki auth login", realm, name)
	}
	if realm == "wikimedia" && a.Endpoint != wikiauth.WikimediaAuthority {
		return a, fmt.Errorf("invalid Wikimedia credential authority")
	}
	if a.Method == "oauth" && (realm != "wikimedia" || a.ClientID == "") {
		return a, fmt.Errorf("OAuth is currently supported only for Wikimedia accounts")
	}
	return a, nil
}

func (a WikiAccount) Credential() Credential {
	prefix, _ := WikiAccountPrefix(a.Realm, a.Name)
	leaf := "password"
	if a.Method == "oauth" {
		leaf = "token"
	}
	return Credential{Key: prefix + leaf, Service: "wiki", Endpoint: a.Endpoint, Account: a.Username}
}

func (a WikiAccount) Metadata() map[string]string {
	prefix, _ := WikiAccountPrefix(a.Realm, a.Name)
	return map[string]string{prefix + "username": a.Username, prefix + "method": a.Method, prefix + "endpoint": a.Endpoint, prefix + "wiki": a.Wiki, prefix + "client_id": a.ClientID, prefix + "redirect_uri": a.RedirectURI, "wiki.realms." + a.Realm + ".default_account": a.Name, "wiki.default_realm": a.Realm}
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
