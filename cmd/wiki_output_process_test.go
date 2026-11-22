package cmd

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWikiWriteProcessStdout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil || r.Form.Get("action") != "wbeditentity" {
			t.Errorf("unexpected wiki request: %v, %v", r.Form, err)
			http.Error(w, "unexpected wiki request", http.StatusBadRequest)
			return
		}
		id := "P123"
		if r.Form.Get("new") == "item" {
			id = "Q123"
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"success":1,"entity":{"id":%q}}`, id)
	}))
	defer server.Close()

	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{"property id", []string{"ext", "wikibase", "property", "put"}, "P123\n"},
		{"item id", []string{"ext", "wikibase", "item", "put", "--property", "P1"}, "Q123\n"},
		{"property preview", []string{"ext", "wikibase", "property", "put", "--dry-run"}, "Dry run mode:"},
		{"item preview", []string{"ext", "wikibase", "item", "put", "--property", "P1", "--dry-run"}, "Dry run mode:"},
		{"page preview", []string{"page", "put", "--title", "Sandbox", "--dry-run"}, "Dry run mode:"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("MWCLI_CONTEXT_TEST", "1")
			args := append([]string{"--no-interaction", "wiki"}, test.args...)
			args = append(args, "--wiki", server.URL+"/w/api.php", "--anon")
			process := cliTestProcess(t, t.Context(), t.TempDir(), args...)
			var stderr bytes.Buffer
			process.Stderr = &stderr
			stdout, err := process.Output()
			if err != nil || !strings.HasPrefix(string(stdout), test.want) || stderr.Len() != 0 {
				t.Fatalf("expected %q on stdout, got stdout %q, stderr %q, error %v", test.want, stdout, &stderr, err)
			}
		})
	}
}
