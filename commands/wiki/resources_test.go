package wiki

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPageListSourcesAndOutputs(t *testing.T) {
	isolateWikiProfiles(t)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		q := r.URL.Query()
		module := q.Get("list")
		switch module {
		case "search":
			if q.Get("srsearch") != "Earth" || q.Get("srnamespace") != "0" {
				t.Error(q)
			}
		case "categorymembers":
			if q.Get("cmtitle") != "Category:Physics" {
				t.Error(q)
			}
		case "allpages":
			if q.Get("apprefix") != "E" || q.Get("apdir") != "descending" || q.Get("apminsize") != "12" {
				t.Error(q)
			}
		default:
			t.Errorf("unexpected source %q", module)
		}
		fmt.Fprintf(w, `{"query":{%q:[{"pageid":1,"ns":0,"title":"Earth"},{"pageid":2,"ns":0,"title":"Mars"}]}}`, module)
	}))
	defer server.Close()
	common := []string{"--anon", "--wiki", server.URL + "/w/api.php"}
	args := append([]string{"page", "list", "--search", "Earth", "--namespace", "0", "--limit", "1"}, common...)
	if out, err := runWikiProfile(t, args, ""); err != nil || out != "Earth\n" {
		t.Fatalf("%q %v", out, err)
	}
	args = append([]string{"page", "list", "--category", "Physics", "--limit", "1", "--output", "table"}, common...)
	if out, err := runWikiProfile(t, args, ""); err != nil || !strings.Contains(out, "PAGEID") || !strings.Contains(out, "Earth") {
		t.Fatalf("table: %q %v", out, err)
	}
	args = append([]string{"page", "list", "--prefix", "E", "--dir", "DESC", "--minsize", "12", "--limit", "1", "--output", "json"}, common...)
	if out, err := runWikiProfile(t, args, ""); err != nil || !strings.Contains(out, `"title":"Earth"`) {
		t.Fatalf("legacy filters: %q %v", out, err)
	}
	before := calls
	for _, extra := range [][]string{{"--search", ""}, {"--category", "Physics", "--prefix", "E"}, {"--search", "Earth", "--backlinks", "Mars"}, {"--limit", "-1"}} {
		args = append(append([]string{"page", "list"}, extra...), common...)
		if _, err := runWikiProfile(t, args, ""); err == nil {
			t.Errorf("unsupported/conflicting selectors accepted: %v", extra)
		}
	}
	args = append([]string{"page", "list", "--search", "Earth", "--dry-run"}, common...)
	if _, err := runWikiProfile(t, args, ""); err != nil {
		t.Fatal(err)
	}
	if calls != before {
		t.Fatal("invalid selector or dry-run contacted wiki")
	}
}

func TestEntityResourceCommandsShareReadsAndWrites(t *testing.T) {
	isolateWikiProfiles(t)
	tokens, writes, reads := 0, 0, 0
	var lastAction, lastValue, lastLanguage string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		q := r.Form
		switch q.Get("action") {
		case "wbgetentities":
			reads++
			id := q.Get("ids")
			kind := "item"
			if strings.HasPrefix(id, "P") {
				kind = "property"
			}
			fmt.Fprintf(w, `{"entities":{%q:{"id":%q,"type":%q,"lastrevid":12,"labels":{"en":{"language":"en","value":"Old"}},"descriptions":{"en":{"language":"en","value":"Old description"}},"aliases":{"en":[{"language":"en","value":"Old alias"}]}}}}`, id, id, kind)
		case "wbsearchentities":
			id := "Q42"
			if q.Get("type") == "property" {
				id = "P31"
			}
			fmt.Fprintf(w, `{"search":[{"id":%q,"label":"Example","description":"A search match"}]}`, id)
		case "query":
			tokens++
			fmt.Fprint(w, `{"query":{"tokens":{"csrftoken":"fixture-csrf"}}}`)
		case "wbeditentity":
			writes++
			var data map[string]json.RawMessage
			if json.Unmarshal([]byte(q.Get("data")), &data) != nil {
				t.Error("invalid entity JSON")
			}
			if q.Get("new") != "property" || string(data["datatype"]) != `"string"` {
				t.Error(q)
			}
			if q.Get("token") != "fixture-csrf" || q.Get("summary") != "Create property" {
				t.Error(q)
			}
			fmt.Fprint(w, `{"entity":{"id":"P99","type":"property","lastrevid":13},"success":1}`)
		case "wbsetlabel", "wbsetdescription", "wbsetaliases":
			writes++
			lastAction, lastValue, lastLanguage = q.Get("action"), q.Get("value"), q.Get("language")
			if q.Get("baserevid") != "12" || q.Get("token") != "fixture-csrf" || q.Get("summary") == "" {
				t.Errorf("edit metadata missing: %v", q)
			}
			fmt.Fprintf(w, `{"entity":{"id":%q,"lastrevid":13},"success":1}`, q.Get("id"))
		default:
			t.Errorf("unexpected action: %v", q)
		}
	}))
	defer server.Close()
	common := []string{"--anon", "--wiki", server.URL + "/w/api.php"}
	run := func(args []string, input string) (string, error) {
		return runWikiProfile(t, append(append([]string{"ext", "wikibase"}, args...), common...), input)
	}
	if out, err := run([]string{"item", "get", "q42", "--language", "en,de", "--language-fallback"}, ""); err != nil || !strings.Contains(out, `"id":"Q42"`) {
		t.Fatalf("entity get: %s %v", out, err)
	}
	if out, err := run([]string{"property", "search", "Example", "--output", "table"}, ""); err != nil || !strings.Contains(out, "P31") {
		t.Fatalf("property search: %s %v", out, err)
	}
	if out, err := run([]string{"item", "label", "get", "Q42", "--language", "en", "--language-fallback"}, ""); err != nil || !strings.Contains(out, `"value":"Old"`) {
		t.Fatalf("term get: %s %v", out, err)
	}
	if out, err := run([]string{"property", "create", "--input", "-", "--summary", "Create property"}, `{"datatype":"string","labels":{"en":{"language":"en","value":"Example"}}}`); err != nil || !strings.Contains(out, `"id":"P99"`) {
		t.Fatalf("property create: %s %v", out, err)
	}
	if _, err := run([]string{"item", "label", "set", "Q42", "New label", "--language", "en", "--summary", "Update label"}, ""); err != nil {
		t.Fatal(err)
	}
	if lastAction != "wbsetlabel" || lastValue != "New label" || lastLanguage != "en" {
		t.Fatal(lastAction, lastValue, lastLanguage)
	}

	if _, err := run([]string{"item", "description", "remove", "Q42", "--language", "en"}, ""); err != nil {
		t.Fatal(err)
	}
	if lastAction != "wbsetdescription" || lastValue != "" {
		t.Fatal("description was not removed")
	}
	if _, err := run([]string{"item", "alias", "add", "Q42", "New alias", "Another alias", "--language", "en"}, ""); err != nil {
		t.Fatal(err)
	}
	if lastAction != "wbsetaliases" {
		t.Fatal(lastAction)
	}
	beforeTokens, beforeWrites := tokens, writes
	if out, err := run([]string{"item", "label", "set", "Q42", "Dry label", "--dry-run"}, ""); err != nil || !strings.Contains(out, `"baserevid":12`) {
		t.Fatalf("dry-run: %s %v", out, err)
	}
	if _, err := run([]string{"item", "create", "--input", "-", "--dry-run"}, `{"labels":{"en":{"language":"en","value":"New"}}}`); err != nil {
		t.Fatal(err)
	}
	if tokens != beforeTokens || writes != beforeWrites {
		t.Fatal("dry-run fetched CSRF or wrote")
	}
	if _, err := run([]string{"item", "label", "set", "Q42", "Conflict", "--baserevid", "11"}, ""); err == nil || !strings.Contains(err.Error(), "expected 11") {
		t.Fatalf("revision mismatch accepted: %v", err)
	}
	if tokens != beforeTokens || writes != beforeWrites {
		t.Fatal("conflict fetched CSRF or wrote")
	}
	beforeReads := reads
	for _, args := range [][]string{{"item", "get", "P31"}, {"item", "create", "--input", "-"}, {"property", "create", "--input", "-"}, {"item", "label", "set", "Q42", "", "--language", "en"}, {"item", "label", "set", "Q42", "Example", "--language", "en|de"}} {
		if _, err := run(args, `{"id":"Q42"}`); err == nil {
			t.Errorf("invalid entity operation accepted: %v", args)
		}
	}
	if reads != beforeReads || writes != beforeWrites {
		t.Fatal("invalid operations contacted or wrote to wiki")
	}
}

func TestAuthenticatedResourceRequestsAssertUserAndHonorRawOverride(t *testing.T) {
	isolateWikiProfiles(t)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		expected := "user"
		if calls == 2 {
			expected = "anon"
		}
		if calls == 3 {
			expected = ""
		}
		if r.URL.Query().Get("assert") != expected {
			t.Errorf("request %d assertion: %v", calls, r.URL.Query())
		}
		if calls < 3 && r.Header.Get("Authorization") != "Bearer api-fixture" {
			t.Error("authenticated resource lost OAuth token")
		}
		if calls == 3 && r.Header.Get("Authorization") != "" {
			t.Error("--anon sent a token")
		}
		if r.URL.Query().Get("action") == "wbgetentities" {
			fmt.Fprint(w, `{"entities":{"Q42":{"id":"Q42","type":"item","lastrevid":12}}}`)
		} else {
			fmt.Fprint(w, `{"query":{}}`)
		}
	}))
	defer server.Close()
	seedAPIOAuth(t, server.URL+"/w/api.php")
	for _, args := range [][]string{{"ext", "wikibase", "item", "get", "Q42"}, {"api", "query", "-f", "assert=anon"}, {"ext", "wikibase", "item", "get", "Q42", "--anon"}} {
		if _, err := runWikiProfile(t, args, ""); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 3 {
		t.Fatal(calls)
	}
}
