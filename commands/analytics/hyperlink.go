package analytics

import (
	"net/url"
	"strings"
	"unicode"
)

func rankingPageURL(metric, project, title string) string {
	if (metric != "top-pages" && metric != "top-edited") || title == "" || strings.ContainsFunc(title, unicode.IsControl) {
		return ""
	}
	host := strings.ToLower(project)
	if !strings.HasSuffix(host, ".org") {
		host += ".org"
	}
	// Project families and aggregates aren't individual wikis. Restrict links
	// to Wikimedia wiki hosts rather than treating arbitrary API values as URLs.
	wiki := false
	for _, family := range []string{"wikipedia", "wiktionary", "wikibooks", "wikinews", "wikiquote", "wikisource", "wikiversity", "wikivoyage", "wikimedia", "wikidata", "mediawiki", "wikifunctions"} {
		wiki = wiki || strings.HasSuffix(host, "."+family+".org")
	}
	if !wiki {
		return ""
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return ""
		}
		for _, ch := range label {
			if !(ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '-') {
				return ""
			}
		}
	}
	return (&url.URL{Scheme: "https", Host: host, Path: "/wiki/" + strings.ReplaceAll(title, " ", "_")}).String()
}
