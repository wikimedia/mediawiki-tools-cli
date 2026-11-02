package wiki

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLegacyEntityPutReturnsErrors(t *testing.T) {
	for _, kind := range []string{"item", "property"} {
		t.Run(kind, func(t *testing.T) {
			isolateWikiProfiles(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if err := r.ParseForm(); err != nil {
					t.Error(err)
				}
				if r.Form.Get("action") == "query" {
					fmt.Fprint(w, `{"query":{"tokens":{"csrftoken":"fixture-token"}}}`)
					return
				}
				fmt.Fprint(w, `{"error":{"code":"permissiondenied","info":"Cannot edit"}}`)
			}))
			defer server.Close()
			cmd := NewWikiCmd()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{"ext", "wikibase", kind, "put", "--anon", "--wiki", server.URL + "/w/api.php"})
			if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "permissiondenied") {
				t.Fatalf("expected returned permission error, got %v", err)
			}
		})
	}
}

func TestLegacyWriteValidationAndPreview(t *testing.T) {
	for _, args := range [][]string{
		{"page", "put", "--title", "Test", "--createonly", "--nocreate", "--dry-run"},
		{"page", "put", "--dry-run"},
		{"page", "put", "unexpected", "--title", "Test", "--dry-run"},
		{"ext", "wikibase", "item", "put", "unexpected", "--dry-run"},
		{"ext", "wikibase", "property", "put", "unexpected", "--dry-run"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			isolateWikiProfiles(t)
			cmd := NewWikiCmd()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs(append(args, "--wiki", "https://example.invalid/w/api.php", "--anon"))
			if err := cmd.Execute(); err == nil {
				t.Fatal("expected validation failure")
			}
		})
	}
	for _, kind := range []string{"item", "property"} {
		t.Run(kind+" preview", func(t *testing.T) {
			isolateWikiProfiles(t)
			cmd := NewWikiCmd()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{"ext", "wikibase", kind, "put", "--wiki", "https://example.invalid/w/api.php", "--anon", "--dry-run"})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), "Dry run mode") {
				t.Fatalf("preview missing from command output: %q", out.String())
			}
		})
	}
}
