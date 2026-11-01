package wikiapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiauth"
)

func TestActionEncodingErrorsAndRESTScope(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("User-Agent") == "" {
			t.Error("missing User-Agent")
		}
		if r.URL.Path == "/custom/rest.php/v1/page/Earth/bare" {
			_, _ = io.WriteString(w, `{"title":"Earth"}`)
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("action") == "modernraw" {
			_, _ = io.WriteString(w, `{"errors":[{"code":"badvalue","message":{"key":"apierror-badvalue","params":["titles"]}}]}`)
			return
		}
		if r.Form.Get("action") == "modern" {
			_, _ = io.WriteString(w, `{"errors":[{"code":"badvalue","text":"Modern diagnostic detail"}]}`)
			return
		}
		if r.Form.Get("action") == "fail" {
			_, _ = io.WriteString(w, `{"error":{"code":"permissiondenied","info":"Not allowed"}}`)
			return
		}
		if r.Method != "POST" || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" || r.Form.Get("text") != "x & ☃ | +" {
			t.Errorf("incorrect form request: %s %v", r.Method, r.Form)
		}
		_, _ = io.WriteString(w, `{"edit":{"result":"Success"}}`)
	}))
	defer server.Close()
	client, err := New(server.URL+"/custom/api.php", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Action(context.Background(), "POST", url.Values{"action": {"edit"}, "text": {"x & ☃ | +"}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Action(context.Background(), "GET", url.Values{"action": {"fail"}})
	var apiError *APIError
	if !errors.As(err, &apiError) || apiError.Code != "permissiondenied" {
		t.Fatalf("API error lost: %v", err)
	}
	_, err = client.Action(context.Background(), "GET", url.Values{"action": {"modern"}})
	if err == nil || !strings.Contains(err.Error(), "Modern diagnostic detail") {
		t.Fatalf("modern diagnostic lost: %v", err)
	}
	_, err = client.Action(context.Background(), "GET", url.Values{"action": {"modernraw"}})
	if err == nil || !strings.Contains(err.Error(), "apierror-badvalue") {
		t.Fatalf("raw message diagnostic lost: %v", err)
	}
	_, err = client.REST(context.Background(), "GET", "/v1/page/Earth/bare", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	before := calls
	for _, route := range []string{"https://example.org/w/rest.php/v1/page/X", "//example.org/a", "../api.php", "v1/../../other/rest.php/a", "v1/%2e%2e/a", "v1/%252e%252e/a", "v1/a#fragment", "v1/a\\b"} {
		if _, err = client.REST(context.Background(), "GET", route, nil, nil, nil); err == nil {
			t.Errorf("accepted unsafe route %q", route)
		}
	}
	if before != calls {
		t.Fatal("unsafe paths contacted server")
	}
}

func TestBearerRESTOptInAndNoRedirect(t *testing.T) {
	leaked := false
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked = true }))
	defer other.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture" {
			t.Error("REST request was not authenticated")
		}
		http.Redirect(w, r, other.URL, http.StatusFound)
	}))
	defer server.Close()
	endpoint := server.URL + "/w/api.php"
	h := server.Client()
	h.Transport = wikiauth.BearerTransport{Endpoint: endpoint, Token: "fixture", IncludeREST: true}
	client, err := New(endpoint, h)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.REST(context.Background(), "GET", "v1/page/Earth", nil, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "302") || leaked {
		t.Fatalf("redirect was followed: %v", err)
	}
	for _, p := range []string{"/other/rest.php/a", "/w/rest.php.evil/a", "/w/assets/a", "/w/rest.php/../a", "/w/rest.php/%252e/a"} {
		u, _ := url.Parse(server.URL + p)
		if wikiauth.AllowsAPIRequest(endpoint, u, true) {
			t.Errorf("credentials allowed at %s", p)
		}
	}
}
