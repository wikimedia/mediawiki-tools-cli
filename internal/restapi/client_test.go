package restapi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestPreservesBasePathEscapingAndHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/api/rest_v1/metrics/A%2FB" || r.URL.Query().Get("q") != "a b" || r.Header.Get("Authorization") != "Bearer test" || r.Header.Get("User-Agent") == "" {
			t.Errorf("unexpected request: %s", r.URL)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	resp, err := Request(context.Background(), http.MethodGet, server.URL+"/api/rest_v1/", "/metrics/A%2FB?q=a+b", http.Header{"Authorization": {"Bearer test"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}

func TestRequestDoesNotFollowAuthenticatedRedirects(t *testing.T) {
	reached := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/target" {
			reached = true
		}
		http.Redirect(w, r, "/target", http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	resp, err := Request(context.Background(), http.MethodPost, server.URL, "start", http.Header{"Authorization": {"Bearer test"}}, strings.NewReader("body"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if reached || resp.StatusCode != http.StatusTemporaryRedirect {
		t.Fatal("redirect followed")
	}
}

func TestInvalidURLsAndAPIErrorDetails(t *testing.T) {
	for _, endpoint := range []string{"//elsewhere.example/path", "file:///tmp/data", "https://user:secret@example.org/", "https://example.org/#fragment"} {
		if resp, err := Request(context.Background(), http.MethodGet, BaseURL, endpoint, nil, nil); err == nil {
			resp.Body.Close()
			t.Fatalf("invalid endpoint accepted: %q", endpoint)
		}
	}
	resp := &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": {"60"}}, Body: io.NopCloser(strings.NewReader("rate limited"))}
	err := CheckStatus(resp)
	if err == nil || !strings.Contains(err.Error(), "429") || !strings.Contains(err.Error(), "Retry-After: 60") || !strings.Contains(err.Error(), "rate limited") {
		t.Fatalf("missing error details: %v", err)
	}
}
