package analytics

import "testing"

func TestRankingPageLinks(t *testing.T) {
	for _, tc := range []struct{ metric, project, title, want string }{
		{"top-pages", "en.wikipedia.org", "GBU-43/B_MOAB", "https://en.wikipedia.org/wiki/GBU-43/B_MOAB"},
		{"top-pages", "en.wikipedia", "Albert Einstein", "https://en.wikipedia.org/wiki/Albert_Einstein"},
		{"top-edited", "fr.wikipedia.org", "Café ?#%", "https://fr.wikipedia.org/wiki/Caf%C3%A9_%3F%23%25"},
		{"top-pages", "commons.wikimedia.org", "File:Example.jpg", "https://commons.wikimedia.org/wiki/File:Example.jpg"},
		{"top-pages", "en.wikipedia.org.attacker.test", "Page", ""},
		{"top-pages", "en.wikipedia.org/evil", "Page", ""},
		{"top-pages", "all-projects", "Page", ""},
		{"top-pages", "wikipedia", "Page", ""},
		{"top-pages", "en.wikipedia.org", "Page\x1b", ""},
		{"top", "en.wikipedia.org", "a/b/File.jpg", ""},
	} {
		if got := rankingPageURL(tc.metric, tc.project, tc.title); got != tc.want {
			t.Errorf("%s %s %q: got %q want %q", tc.metric, tc.project, tc.title, got, tc.want)
		}
	}
}
