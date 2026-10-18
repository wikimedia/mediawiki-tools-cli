package traefik

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestLoginLinkUsesAuthenticatedPOST(t *testing.T) {
	var origin string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if r.Method != http.MethodPost || r.URL.Path != "/_mwcli/login-link" || r.URL.RawQuery != "" || !ok || username != "alice" || password != "secret" {
			t.Errorf("unexpected login-link request: %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"url": origin + "/_mwcli/login#login=temporary-code"})
	}))
	defer server.Close()
	origin = server.URL
	link, err := requestLoginLink(origin+"/", "Username: alice\nPassword: secret\n")
	if err != nil || link != origin+"/_mwcli/login#login=temporary-code" {
		t.Fatalf("incorrect login link: %q, %v", link, err)
	}
}

func TestRequestLoginLinkRejectsRedirectsAndForeignLinks(t *testing.T) {
	for _, redirect := range []bool{false, true} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if redirect {
				http.Redirect(w, r, "https://elsewhere.example", http.StatusTemporaryRedirect)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"url": "https://elsewhere.example/_mwcli/login#login=code"})
		}))
		_, err := requestLoginLink(server.URL, "Username: alice\nPassword: secret\n")
		server.Close()
		if err == nil {
			t.Fatal("accepted a redirect or foreign login link")
		}
	}
	for _, origin := range []string{"file:///tmp/gateway", "http://alice:secret@localhost:8080", "https://example.test/path", "https://example.test?secret=foo"} {
		if _, err := requestLoginLink(origin, "Username: alice\nPassword: secret\n"); err == nil {
			t.Fatalf("accepted invalid gateway origin %q", origin)
		}
	}
}
