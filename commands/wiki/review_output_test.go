package wiki

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInvalidResourceFiltersFailBeforeRequestsAndRuntimeFiltersFail(t *testing.T) {
	isolateWikiProfiles(t)
	requests := 0
	posts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_ = r.ParseForm()
		if r.Method == "POST" {
			posts++
			fmt.Fprint(w, `{"success":1,"entity":{"id":"Q1"}}`)
			return
		}
		if r.Form.Get("meta") == "tokens" {
			fmt.Fprint(w, `{"query":{"tokens":{"csrftoken":"fixture"}}}`)
		} else if r.Form.Get("action") == "wbgetentities" {
			fmt.Fprint(w, `{"entities":{"Q1":{"id":"Q1","lastrevid":12}}}`)
		} else {
			fmt.Fprint(w, `{"query":{"search":[]}}`)
		}
	}))
	defer server.Close()
	base := []string{"--anon", "--wiki", server.URL + "/w/api.php"}
	run := func(args []string) (string, error) {
		cmd := NewWikiCmd()
		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs(args)
		err := cmd.Execute()
		return stdout.String(), err
	}
	for _, args := range [][]string{
		{"page", "list", "--search", "Earth", "--output", "jq", "--format", "["},
		{"api", "query", "--output", "json", "--format", "["},
		{"api", "query", "--output", "table"},
		{"page", "list", "--output", "web"},
		{"ext", "wikibase", "item", "label", "set", "Q1", "Earth", "--language", "en", "--output", "jq", "--format", "["},
	} {
		out, err := run(append(append([]string{}, base...), args...))
		if err == nil || out != "" {
			t.Fatalf("%v: %q %v", args, out, err)
		}
	}
	if requests != 0 {
		t.Fatal("invalid syntax contacted wiki", requests)
	}
	out, err := run(append(append([]string{}, base...), "page", "list", "--search", "Earth", "--output", "jq", "--format", `error("fixture")`))
	if err == nil || out != "" {
		t.Fatalf("%q %v", out, err)
	}
	out, err = run(append(append([]string{}, base...), "ext", "wikibase", "item", "label", "set", "Q1", "Earth", "--language", "en", "--output", "jq", "--format", `error("fixture")`))
	if err == nil || !strings.Contains(err.Error(), "edit succeeded, but output failed") || out != "" || posts != 1 {
		t.Fatalf("%q posts=%d %v", out, posts, err)
	}
}
