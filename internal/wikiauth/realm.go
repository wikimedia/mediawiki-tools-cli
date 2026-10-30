// Package wikiauth implements wiki authentication without embedding client secrets.
package wikiauth

import (
	_ "embed"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

const WikimediaAuthority = "https://meta.wikimedia.org"

//go:embed wikimedia.txt
var wikimediaSites string

var names = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

func ValidName(name string) bool { return names.MatchString(name) }

func Endpoint(raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("provide a wiki API endpoint")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid wiki endpoint")
	}
	if u.Path == "" || u.Path == "/" {
		u.Path = "/w/api.php"
	}
	endpoint, err := secrets.Endpoint(u.String())
	if err != nil {
		return "", err
	}
	if !strings.HasSuffix(u.Path, "/api.php") {
		return "", fmt.Errorf("wiki endpoint must end in /api.php")
	}
	return endpoint, nil
}

func IsWikimedia(endpoint string) bool {
	for _, line := range strings.Split(wikimediaSites, "\n") {
		if line == endpoint {
			return true
		}
	}
	return false
}

// Exact endpoints, never domain suffixes, determine credential destinations.
func Allows(realm, endpoint string, custom []string) bool {
	if realm == "wikimedia" {
		return IsWikimedia(endpoint)
	}
	for _, allowed := range custom {
		if endpoint == allowed {
			return true
		}
	}
	return false
}
