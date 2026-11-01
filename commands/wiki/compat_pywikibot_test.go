package wiki

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiapi"
)

// These tests cover the removable syntax adapter, not canonical selectors.
func TestPywikibotCompatibilitySupportedSubset(t *testing.T) {
	for _, tc := range []struct{ argument, source, value string }{{"-search:Earth", "search", "Earth"}, {"-cat:Physics", "category", "Physics"}, {"-ref:Earth", "backlinks", "Earth"}, {"-backlinks:Earth", "backlinks", "Earth"}, {"-transcludes:Template:X", "transclusions", "Template:X"}} {
		t.Run(tc.argument, func(t *testing.T) {
			cmd := NewWikiPageListCmd()
			_ = cmd.Flags().Set("pywikibot", tc.argument)
			_ = cmd.Flags().Set("pywikibot", "-namespace:0")
			_ = cmd.Flags().Set("pywikibot", "-limit:2")
			compatibility := pywikibotSelectorCompatibility{args: []string{tc.argument, "-namespace:0", "-limit:2"}}
			var options wikiapi.PageSelection
			if err := compatibility.apply(cmd, &options); err != nil || options.Source != tc.source || options.Value != tc.value || options.Limit != 2 || options.Namespace == nil || *options.Namespace != 0 {
				t.Fatalf("%+v %v", options, err)
			}
		})
	}
}

func TestPywikibotCompatibilityRejectsUnsupportedVariants(t *testing.T) {
	for _, args := range [][]string{{"-catr:Physics"}, {"-cat:Physics|Start"}, {"-cat:Physics#Start"}, {"-namespace:Main"}, {"-namespace:0,1"}, {"-search:Earth", "-ref:Moon"}, {"-file:titles.txt"}, {"-simulate"}, {"-lang:en"}} {
		var options wikiapi.PageSelection
		if err := applyPywikibotSelectors(&options, args, false, false); err == nil {
			t.Errorf("unsupported syntax accepted: %v", args)
		}
	}
}

func TestPywikibotCompatibilityCLIUsesCanonicalReadPath(t *testing.T) {
	isolateWikiProfiles(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("list") != "backlinks" || q.Get("bltitle") != "Earth" || q.Get("blnamespace") != "0" {
			t.Error(q)
		}
		fmt.Fprint(w, `{"query":{"backlinks":[{"pageid":1,"ns":0,"title":"Moon"}]}}`)
	}))
	defer server.Close()
	out, err := runWikiProfile(t, []string{"page", "list", "--pywikibot=-ref:Earth", "--pywikibot=-namespace:0", "--anon", "--wiki", server.URL + "/w/api.php"}, "")
	if err != nil || strings.TrimSpace(out) != "Moon" {
		t.Fatalf("%q %v", out, err)
	}
}
