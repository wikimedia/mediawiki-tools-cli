package main

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testOrigin = "https://gateway.example"

func testGateway(t *testing.T) (*gateway, string) {
	t.Helper()
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "credentials"), []byte("Username: alice\nPassword: secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "session-key"), bytes.Repeat([]byte{1}, 32), 0o600); err != nil {
		t.Fatal(err)
	}
	g, err := newGateway(directory)
	if err != nil {
		t.Fatal(err)
	}
	g.now = func() time.Time { return time.Unix(1800000000, 0) }
	return g, directory
}

func request(method, path string, form url.Values, cookies ...*http.Cookie) *http.Request {
	var body string
	if form != nil {
		body = form.Encode()
	}
	r := httptest.NewRequest(method, testOrigin+path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("X-Forwarded-Host", "gateway.example")
	r.Header.Set("X-Forwarded-Proto", "https")
	for _, cookie := range cookies {
		r.AddCookie(cookie)
	}
	return r
}

func cookieFrom(t *testing.T, recorder *httptest.ResponseRecorder, purpose string) *http.Cookie {
	t.Helper()
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == cookieName(testOrigin, purpose) {
			return cookie
		}
	}
	t.Fatalf("missing %s cookie", purpose)
	return nil
}

func login(t *testing.T, g *gateway) *http.Cookie {
	t.Helper()
	get := httptest.NewRecorder()
	g.ServeHTTP(get, request(http.MethodGet, loginPath, nil))
	csrf := cookieFrom(t, get, "csrf")
	form := url.Values{"username": {"alice"}, "password": {"secret"}, "csrf": {csrf.Value}, "return": {"/proxy/keycloak/admin/"}}
	post := httptest.NewRecorder()
	g.ServeHTTP(post, request(http.MethodPost, loginPath, form, csrf))
	if post.Code != http.StatusSeeOther || post.Header().Get("Location") != "/proxy/keycloak/admin/" {
		t.Fatalf("login: %d %s", post.Code, post.Body.String())
	}
	return cookieFrom(t, post, "session")
}

func TestSessionProtectsGatewayAndPreservesApplicationAuth(t *testing.T) {
	g, _ := testGateway(t)
	session := login(t, g)
	if !session.Secure || !session.HttpOnly || session.SameSite != http.SameSiteLaxMode || session.Path != "/" || session.Domain != "" || session.MaxAge != int(sessionLifetime.Seconds()) {
		t.Fatalf("unsafe session cookie: %+v", session)
	}
	for _, tc := range []struct{ name, header, want string }{
		{"no app credentials", "", ""},
		{"bearer", "Bearer application-access-token", "Bearer application-access-token"},
		{"native basic", "Basic " + base64.StdEncoding.EncodeToString([]byte("app:password")), "Basic " + base64.StdEncoding.EncodeToString([]byte("app:password"))},
		{"gateway basic", "Basic " + base64.StdEncoding.EncodeToString([]byte("alice:secret")), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := request(http.MethodGet, "/check", nil, session)
			r.Header.Set("Authorization", tc.header)
			w := httptest.NewRecorder()
			g.ServeHTTP(w, r)
			if w.Code != http.StatusNoContent || w.Header().Get("Authorization") != tc.want {
				t.Fatalf("auth: %d, %q", w.Code, w.Header().Get("Authorization"))
			}
		})
	}
	g.now = func() time.Time { return time.Unix(1800000000, 0).Add(sessionLifetime) }
	w := httptest.NewRecorder()
	g.ServeHTTP(w, request(http.MethodGet, "/check", nil, session))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expired cookie accepted: %d", w.Code)
	}
}

func TestAnonymousRequestsAndScriptCredentials(t *testing.T) {
	g, _ := testGateway(t)
	for _, tc := range []struct {
		name, accept, authorization string
		status                      int
	}{
		{"API", "application/json", "", http.StatusUnauthorized},
		{"browser", "text/html", "", http.StatusSeeOther},
		{"unverified bearer", "application/json", "Bearer anything", http.StatusUnauthorized},
		{"wrong basic", "application/json", "Basic " + base64.StdEncoding.EncodeToString([]byte("alice:wrong")), http.StatusUnauthorized},
		{"script basic", "application/json", "Basic " + base64.StdEncoding.EncodeToString([]byte("alice:secret")), http.StatusNoContent},
		{"browser cached basic needs cookie", "text/html", "Basic " + base64.StdEncoding.EncodeToString([]byte("alice:secret")), http.StatusSeeOther},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := request(http.MethodGet, "/check", nil)
			r.Header.Set("Accept", tc.accept)
			r.Header.Set("Authorization", tc.authorization)
			r.Header.Set("X-Forwarded-Method", http.MethodGet)
			r.Header.Set("X-Forwarded-Uri", "/proxy/keycloak/admin/?page=clients")
			w := httptest.NewRecorder()
			g.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			if w.Header().Get("Authorization") != "" {
				t.Fatal("gateway credentials leaked")
			}
			if tc.status == http.StatusSeeOther && !strings.HasPrefix(w.Header().Get("Location"), loginPath+"?return=") {
				t.Fatal("login redirect lost")
			}
		})
	}
}

func TestSignedTicketsRejectTamperingAndDifferentOrigins(t *testing.T) {
	g, directory := testGateway(t)
	value, err := g.sign("session", testOrigin, sessionLifetime)
	if err != nil {
		t.Fatal(err)
	}
	if !g.valid(value, "session", testOrigin) {
		t.Fatal("valid ticket rejected")
	}
	if g.valid("X"+value[1:], "session", testOrigin) || g.valid(value, "csrf", testOrigin) || g.valid(value, "session", "http://gateway.example") || g.valid(value, "session", "https://elsewhere.example") {
		t.Fatal("invalid ticket accepted")
	}
	if err := os.WriteFile(filepath.Join(directory, "credentials"), []byte("Username: alice\nPassword: changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rotated, err := newGateway(directory)
	if err != nil {
		t.Fatal(err)
	}
	rotated.now = g.now
	if rotated.valid(value, "session", testOrigin) {
		t.Fatal("credential change did not invalidate session")
	}
}

func TestFormsRequireCSRFAndLogoutClearsSession(t *testing.T) {
	g, _ := testGateway(t)
	session := login(t, g)
	get := httptest.NewRecorder()
	g.ServeHTTP(get, request(http.MethodGet, logoutPath, nil, session))
	csrf := cookieFrom(t, get, "csrf")
	for _, path := range []string{loginPath, logoutPath} {
		for _, foreignOrigin := range []bool{false, true} {
			r := request(http.MethodPost, path, url.Values{"csrf": {csrf.Value}, "username": {"alice"}, "password": {"secret"}}, session)
			if foreignOrigin {
				r.AddCookie(csrf)
				r.Header.Set("Origin", "https://untrusted.example")
			}
			w := httptest.NewRecorder()
			g.ServeHTTP(w, r)
			if w.Code != http.StatusForbidden {
				t.Fatalf("CSRF allowed for %s: %d", path, w.Code)
			}
		}
	}
	w := httptest.NewRecorder()
	g.ServeHTTP(w, request(http.MethodPost, logoutPath, url.Values{"csrf": {csrf.Value}}, session, csrf))
	if w.Code != http.StatusSeeOther || cookieFrom(t, w, "session").MaxAge != -1 {
		t.Fatal("logout did not clear cookie")
	}
	// GET only displays the confirmation; it never clears a session.
	for _, cookie := range get.Result().Cookies() {
		if cookie.Name == session.Name {
			t.Fatal("GET logged user out")
		}
	}
}

func TestWrongPasswordAndCookieSecurityOnLocalHTTP(t *testing.T) {
	g, _ := testGateway(t)
	get := httptest.NewRecorder()
	g.ServeHTTP(get, request(http.MethodGet, loginPath, nil))
	csrf := cookieFrom(t, get, "csrf")
	w := httptest.NewRecorder()
	g.ServeHTTP(w, request(http.MethodPost, loginPath, url.Values{"csrf": {csrf.Value}, "username": {"alice"}, "password": {"wrong"}}, csrf))
	if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), "incorrect") {
		t.Fatal("wrong password did not fail")
	}
	if g.credentialsMatch("alice\x00secret", "") {
		t.Fatal("ambiguous credentials accepted")
	}
	r := httptest.NewRequest(http.MethodGet, "http://localhost:8080"+loginPath, nil)
	w = httptest.NewRecorder()
	g.ServeHTTP(w, r)
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode {
		t.Fatal("local HTTP cookie is incorrect")
	}
	if cookieName("http://localhost:8080", "session") == cookieName("http://localhost:8081", "session") {
		t.Fatal("contexts share cookie names")
	}
}

func TestReturnURLStaysOnGateway(t *testing.T) {
	for _, path := range []string{"https://evil.example", "//evil.example", "/\\evil.example", "/%2fevil.example", "/%5cevil.example", "javascript:alert(1)", "/_mwcli/login", "", "/\nLocation: evil"} {
		if got := safeReturn(path); got != "/proxy/dashboard/" {
			t.Errorf("unsafe return %q accepted as %q", path, got)
		}
	}
	for _, path := range []string{"/", "/proxy/default/wiki/Main_Page", "/proxy/jaeger/search?service=mediawiki"} {
		if got := safeReturn(path); got != path {
			t.Errorf("return %q changed to %q", path, got)
		}
	}
}

func TestWebSocketUsesTheHTTPLoginSession(t *testing.T) {
	g, _ := testGateway(t)
	for _, tc := range []struct{ protocol, origin string }{{"ws", "http://gateway.example"}, {"wss", testOrigin}} {
		t.Run(tc.protocol, func(t *testing.T) {
			token, err := g.sign("session", tc.origin, sessionLifetime)
			if err != nil {
				t.Fatal(err)
			}
			r := request(http.MethodGet, "/check", nil, &http.Cookie{Name: cookieName(tc.origin, "session"), Value: token})
			r.Header.Set("X-Forwarded-Proto", tc.protocol)
			w := httptest.NewRecorder()
			g.ServeHTTP(w, r)
			if w.Code != http.StatusNoContent {
				t.Fatalf("WebSocket rejected: %d %s", w.Code, w.Body.String())
			}
		})
	}
}
