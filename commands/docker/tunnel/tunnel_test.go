package tunnel

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/pkg/dockercompose"
)

func TestReadURLUsesCurrentContainerRun(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/_ping"):
			w.Header().Set("API-Version", "1.44")
		case strings.HasSuffix(r.URL.Path, "/containers/json"):
			fmt.Fprint(w, `[{"Id":"tunnel-test"}]`)
		case strings.HasSuffix(r.URL.Path, "/containers/tunnel-test/json"):
			fmt.Fprint(w, `{"State":{"StartedAt":"2026-10-18T12:00:00Z"}}`)
		default:
			t.Errorf("unexpected Docker API request: %s", r.URL)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("DOCKER_HOST", strings.Replace(server.URL, "http://", "tcp://", 1))
	t.Setenv("DOCKER_API_VERSION", "1.44")
	t.Setenv("DOCKER_TLS_VERIFY", "")
	t.Setenv("DOCKER_CERT_PATH", "")
	dir := t.TempDir()
	// Simulate Docker retaining an old URL unless logs are scoped to this run.
	script := `#!/bin/sh
case "$*" in
  *"--since 2026-10-18T12:00:00Z"*) printf '%s' "$TUNNEL_TEST_LOG" ;;
  *"logs"*) printf 'https://previous.trycloudflare.com' ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	for _, tt := range []struct{ name, logs, want string }{
		{"waiting for new URL", "Starting tunnel...", ""},
		{"current URL", "https://current.trycloudflare.com", "https://current.trycloudflare.com"},
		{"latest URL", "https://first.trycloudflare.com\nhttps://latest.trycloudflare.com", "https://latest.trycloudflare.com"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TUNNEL_TEST_LOG", tt.logs)
			got, _, err := readURL(dockercompose.Project{Name: "test", Directory: dir})
			if err != nil || got != tt.want {
				t.Fatalf("got %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}
