package wikiapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestEntitySearchPaginationAndMissingEntities(t *testing.T) {
	searches := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("action") == "wbgetentities" {
			fmt.Fprint(w, `{"entities":{"Q42":{"id":"Q42","missing":true}}}`)
			return
		}
		searches++
		if q.Get("type") != "property" || q.Get("language") != "en" {
			t.Error(q)
		}
		if searches == 1 {
			fmt.Fprint(w, `{"search":[{"id":"P1","label":"one"}],"search-continue":1}`)
		} else {
			if q.Get("continue") != "1" {
				t.Error(q)
			}
			fmt.Fprint(w, `{"search":[{"id":"P1"},{"id":"P2","label":"two"}],"search-continue":2}`)
		}
	}))
	defer server.Close()
	client, _ := New(server.URL+"/w/api.php", server.Client())
	results, err := client.SearchEntities(context.Background(), "property", "value", "en", 2)
	if err != nil || len(results) != 2 || searches != 2 {
		t.Fatalf("%v %v", results, err)
	}
	if _, err = client.Entity(context.Background(), "item", "Q42", nil, "", false); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatal(err)
	}
}

func TestEntityTermWriteKeepsRevisionSummaryAndLiteralPipes(t *testing.T) {
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("meta") == "tokens" {
			fmt.Fprint(w, `{"query":{"tokens":{"csrftoken":"fixture-csrf"}}}`)
			return
		}
		writes++
		for key, value := range map[string]string{"action": "wbsetaliases", "id": "Q42", "language": "en", "baserevid": "123", "summary": "Test edit", "bot": "1", "token": "fixture-csrf", "add": "\x1fA | B\x1fAnother"} {
			if r.Form.Get(key) != value {
				t.Errorf("%s: got %q want %q", key, r.Form.Get(key), value)
			}
		}
		if r.Method != "POST" {
			t.Error("edit was not POST")
		}
		fmt.Fprint(w, `{"entity":{"id":"Q42","lastrevid":124},"success":1}`)
	}))
	defer server.Close()
	client, _ := New(server.URL+"/w/api.php", server.Client())
	_, err := client.ChangeTerm(context.Background(), "item", "Q42", "alias", "add", "en", []string{"A | B", "Another"}, EditOptions{Summary: "Test edit", BaseRevision: 123, Bot: true})
	if err != nil || writes != 1 {
		t.Fatalf("%d %v", writes, err)
	}
	if _, err = client.ChangeTerm(context.Background(), "item", "Q42", "label", "set", "en", []string{"New"}, EditOptions{}); err == nil {
		t.Fatal("edit without base revision accepted")
	}
}

func TestEntityCreateValidationAndAuthenticatedAssertions(t *testing.T) {
	for _, tc := range []struct{ kind, data string }{
		{"item", `{"id":"Q42"}`}, {"property", `{"labels":{}}`}, {"item", `[]`}, {"item", `{"type":"property"}`}, {"lexeme", `{}`},
	} {
		if _, err := ValidateEntityCreate(tc.kind, []byte(tc.data)); err == nil {
			t.Errorf("invalid create accepted: %s %s", tc.kind, tc.data)
		}
	}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		want := "user"
		if calls == 2 {
			want = "anon"
		}
		if r.URL.Query().Get("assert") != want {
			t.Error(r.URL.Query())
		}
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer server.Close()
	client, _ := New(server.URL+"/w/api.php", server.Client())
	client.AssertUser = true
	if _, err := client.Action(context.Background(), "GET", url.Values{"action": {"query"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Action(context.Background(), "GET", url.Values{"action": {"query"}, "assert": {"anon"}}); err != nil {
		t.Fatal(err)
	}
	if entity, err := ValidateEntityCreate("property", []byte(`{"datatype":"string","labels":{"en":{"language":"en","value":"A"}}}`)); err != nil || !json.Valid(entity["datatype"]) {
		t.Fatalf("valid property rejected: %v", err)
	}
}
