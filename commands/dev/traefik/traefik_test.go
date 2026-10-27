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
		if r.Method != http.MethodPost || r.URL.Path != "/_wm/login-link" || r.URL.RawQuery != "" || !ok || username != "alice" || password != "secret" {
			t.Errorf("unexpected login-link request: %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"url": origin + "/_wm/login#login=temporary-code"})
	}))
	defer server.Close()
	origin = server.URL
	link, err := requestLoginLink(origin+"/", "Username: alice\nPassword: secret\n")
	if err != nil || link != origin+"/_wm/login#login=temporary-code" {
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
			_ = json.NewEncoder(w).Encode(map[string]string{"url": "https://elsewhere.example/_wm/login#login=code"})
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

func TestRunningTunnelURLOnlyReturnsAnActiveQuickTunnel(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
		status           int
	}{
		{"running", `{"tunnel_url":"https://active.trycloudflare.com"}`, "https://active.trycloudflare.com", http.StatusOK},
		{"stopped or not ready", `{"services":[]}`, "", http.StatusOK},
		{"unavailable dashboard", "unavailable", "", http.StatusServiceUnavailable},
		{"wrong response", "not JSON", "", http.StatusOK},
		{"foreign host", `{"tunnel_url":"https://elsewhere.example"}`, "", http.StatusOK},
		{"insecure URL", `{"tunnel_url":"http://active.trycloudflare.com"}`, "", http.StatusOK},
		{"userinfo", `{"tunnel_url":"https://active.trycloudflare.com@elsewhere.example"}`, "", http.StatusOK},
		{"unexpected path", `{"tunnel_url":"https://active.trycloudflare.com/path"}`, "", http.StatusOK},
		{"redirect", "", "", http.StatusTemporaryRedirect},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/proxy/dashboard/api/status" || r.Header.Get("Authorization") != "" {
					t.Error("unexpected dashboard request")
				}
				if tc.status == http.StatusTemporaryRedirect {
					w.Header().Set("Location", "/should-not-be-followed")
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			if got := runningTunnelURL(server.URL, "Username: alice\nPassword: secret\n"); got != tc.want {
				t.Fatalf("got %q; want %q", got, tc.want)
			}
		})
	}
}

func TestLoginLinkFallsBackOnlyWhenGatewayHasNoNewEndpoint(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusUnauthorized, http.StatusServiceUnavailable} {
		requests := 0
		var origin string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			if r.URL.Path == "/_wm/login-link" {
				w.WriteHeader(status)
				return
			}
			if r.URL.Path != "/_mwcli/login-link" {
				t.Errorf("unexpected route: %s", r.URL.Path)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"url": origin + "/_mwcli/login#login=legacy-code"})
		}))
		origin = server.URL
		link, err := requestLoginLink(origin, "Username: alice\nPassword: secret\n")
		server.Close()
		if status == http.StatusNotFound {
			if err != nil || requests != 2 || link != origin+"/_mwcli/login#login=legacy-code" {
				t.Fatalf("old gateway fallback failed: %q %v (%d requests)", link, err, requests)
			}
		} else if err == nil || requests != 1 {
			t.Fatal("authentication/server failure caused a legacy retry")
		}
	}
}
