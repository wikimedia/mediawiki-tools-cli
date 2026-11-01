package wikiauth

import (
	"net/url"
	"path"
	"strings"
)

// AllowsAPIRequest validates the origin and canonical path before credentials
// can be sent. Sibling installations, assets, path traversal and redirects to
// another origin must not receive cookies or bearer tokens.
func AllowsAPIRequest(endpoint string, target *url.URL, includeREST bool) bool {
	api, err := url.Parse(endpoint)
	if err != nil || target == nil || target.User != nil || target.Fragment != "" || api.Scheme != target.Scheme || !strings.EqualFold(api.Host, target.Host) {
		return false
	}
	p := target.Path
	for i := 0; i < 8; i++ {
		if path.Clean(p) != p || strings.Contains(p, "\\") {
			return false
		}
		decoded, e := url.PathUnescape(p)
		if e != nil || decoded == p {
			break
		}
		p = decoded
		if i == 7 {
			return false
		}
	}
	if target.Path == api.Path {
		return true
	}
	return includeREST && strings.HasPrefix(target.Path, strings.TrimSuffix(api.Path, "api.php")+"rest.php/")
}
