package wiki

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Adapter-specific positional syntax and upstream pipe-list semantics live here.
func TestWikibaseCLICompatibilityInfersTypeAndSplitsAliasLists(t *testing.T) {
	isolateWikiProfiles(t)
	var action, id, language, value, aliases string
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		q := r.Form
		switch q.Get("action") {
		case "wbgetentities":
			kind := "item"
			if strings.HasPrefix(q.Get("ids"), "P") {
				kind = "property"
			}
			fmt.Fprintf(w, `{"entities":{%q:{"id":%q,"type":%q,"lastrevid":12}}}`, q.Get("ids"), q.Get("ids"), kind)
		case "query":
			fmt.Fprint(w, `{"query":{"tokens":{"csrftoken":"fixture"}}}`)
		default:
			writes++
			action, id, language, value = q.Get("action"), q.Get("id"), q.Get("language"), q.Get("value")
			aliases = q.Get("add") + q.Get("remove")
			if q.Get("baserevid") != "12" || q.Get("token") != "fixture" {
				t.Error(q)
			}
			fmt.Fprintf(w, `{"entity":{"id":%q},"success":1}`, id)
		}
	}))
	defer server.Close()
	common := []string{"--anon", "--wiki", server.URL + "/w/api.php"}
	for _, tc := range []struct{ command, id, value, action string }{{"set-label", "P31", "Property label", "wbsetlabel"}, {"set-description", "Q42", "Description", "wbsetdescription"}, {"add-alias", "Q42", "First|Second", "wbsetaliases"}, {"remove-alias", "P31", "First|Second", "wbsetaliases"}} {
		args := append([]string{"ext", "wikibase", tc.command, tc.id, "en", tc.value}, common...)
		if _, err := runWikiProfile(t, args, ""); err != nil {
			t.Fatal(err)
		}
		if action != tc.action || id != tc.id || language != "en" {
			t.Fatal(action, id, language)
		}
		if strings.Contains(tc.command, "alias") {
			if aliases != "First|Second" {
				t.Fatalf("upstream alias list semantics lost: %q", aliases)
			}
		} else if value != tc.value {
			t.Fatal(value)
		}
	}
	before := writes
	if _, err := runWikiProfile(t, append([]string{"ext", "wikibase", "add-alias", "Q42", "en", "First|Second", "--dry-run"}, common...), ""); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"set-label", "L1", "en", "No lexemes"}, {"add-alias", "Q42", "en", "First||Second"}, {"set-label", "Q42", "en", "No templates", "--template", "code.js"}} {
		if _, err := runWikiProfile(t, append(append([]string{"ext", "wikibase"}, args...), common...), ""); err == nil {
			t.Errorf("unsupported upstream behavior accepted: %v", args)
		}
	}
	if writes != before {
		t.Fatal("unsupported syntax or dry-run wrote")
	}
	root := NewWikiExtWikibaseCmd()
	for _, name := range []string{"set-label", "set-description", "add-alias", "remove-alias"} {
		cmd, _, err := root.Find([]string{name})
		if err != nil || !cmd.Hidden {
			t.Fatalf("compatibility redirect %s is not hidden", name)
		}
	}
}
