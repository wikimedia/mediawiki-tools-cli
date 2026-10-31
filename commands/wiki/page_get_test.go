package wiki

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPageGet(t *testing.T) {
	for _, test := range []struct {
		name, response, want, wantError string
		args                            []string
	}{
		{name: "main page", response: `{"query":{"pages":[{"title":"Project:Home","revisions":[{"slots":{"main":{"content":"== Hello ==\n"}}}]}]}}`, want: "== Hello ==\n"},
		{name: "explicit title", args: []string{"--title", "Project:Home"}, response: `{"query":{"pages":[{"revisions":[{"slots":{"main":{"content":"source"}}}]}]}}`, want: "source"},
		{name: "positional title", args: []string{"Project:Home"}, response: `{"query":{"pages":[{"revisions":[{"slots":{"main":{"content":""}}}]}]}}`},
		{name: "missing", args: []string{"Project:Home"}, response: `{"query":{"pages":[{"missing":true}]}}`, wantError: "does not exist"},
		{name: "hidden revision", args: []string{"Project:Home"}, response: `{"query":{"pages":[{}]}}`, wantError: "no readable revision"},
		{name: "API error", args: []string{"Project:Home"}, response: `{"error":{"code":"permissiondenied","info":"Denied"}}`, wantError: "permissiondenied"},
		{name: "conflicting titles", args: []string{"Project:Home", "--title", "Other"}, wantError: "not both"},
	} {
		t.Run(test.name, func(t *testing.T) {
			isolateWikiProfiles(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("unexpected method %s", r.Method)
				}
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Query().Get("meta") == "siteinfo" {
					fmt.Fprint(w, `{"query":{"general":{"mainpage":"Project:Home"}}}`)
					return
				}
				if r.URL.Query().Get("titles") != "Project:Home" {
					t.Errorf("unexpected title: %s", r.URL.Query().Get("titles"))
				}
				fmt.Fprint(w, test.response)
			}))
			defer server.Close()
			cmd := NewWikiCmd()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs(append([]string{"page", "get", "--anon", "--wiki", server.URL + "/w/api.php"}, test.args...))
			err := cmd.Execute()
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("got %v, want error containing %q", err, test.wantError)
				}
				return
			}
			if err != nil || out.String() != test.want {
				t.Fatalf("output %q, error %v; want %q", out.String(), err, test.want)
			}
		})
	}
}
