package wiki

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPageDeleteReturnsAPIErrorsAndStops(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.Form.Get("action") {
		case "query":
			json.NewEncoder(w).Encode(map[string]any{"query": map[string]any{"tokens": map[string]string{"csrftoken": "fixture-token"}}})
		case "delete":
			calls++
			if r.Form.Get("title") != "Cli/ref/first" || r.Form.Get("reason") != "Moved reference" {
				t.Error("wrong deletion parameters")
			}
			json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "permissiondenied", "info": "Not allowed"}})
		default:
			t.Errorf("unexpected API request %v", r.Form)
		}
	}))
	defer server.Close()
	cmd := NewWikiCmd()
	cmd.SetArgs([]string{"page", "delete", "Cli/ref/first", "Cli/ref/second", "--wiki", server.URL + "/w/api.php", "--anon", "--reason", "Moved reference"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "permissiondenied") {
		t.Fatalf("deletion failure was swallowed: %v", err)
	}
	if calls != 1 {
		t.Fatalf("attempted %d deletions after failure", calls)
	}
}
