package wikiauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestPKCEAndCallbackValidation(t *testing.T) {
	a, err := Begin(TestClientID, TestRedirectURI)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(a.URL)
	q := u.Query()
	hash := sha256.Sum256([]byte(a.Verifier))
	if q.Get("code_challenge") != base64.RawURLEncoding.EncodeToString(hash[:]) || q.Get("code_challenge_method") != "S256" || q.Has("client_secret") {
		t.Fatal("invalid public-client PKCE request")
	}
	other, _ := Begin(TestClientID, TestRedirectURI)
	if other.State == a.State || other.Verifier == a.Verifier {
		t.Fatal("login attempts reused randomness")
	}
	valid := TestRedirectURI + "?" + url.Values{"code": {"test-code"}, "state": {a.State}}.Encode()
	if code, err := a.Code(valid); err != nil || code != "test-code" {
		t.Fatal("valid callback rejected")
	}
	for _, bad := range []string{strings.Replace(valid, a.State, "wrong", 1), strings.Replace(valid, "localhost", "evil.example", 1), strings.Replace(valid, "/auth/login", "/other", 1), valid + "&state=" + a.State, valid + "&code=other", valid + "&error=access_denied"} {
		if _, err := a.Code(bad); err == nil {
			t.Fatal("unsafe callback accepted")
		}
	}
}

func TestExchangeAndRefreshWithoutClientSecret(t *testing.T) {
	a, _ := Begin(TestClientID, TestRedirectURI)
	calls := 0
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		_ = r.ParseForm()
		if r.Form.Has("client_secret") || r.Header.Get("Authorization") != "" {
			t.Error("public client sent client authentication")
		}
		if r.Form.Get("client_id") != TestClientID {
			t.Error("missing client identity")
		}
		if calls == 1 && (r.Form.Get("code_verifier") != a.Verifier || r.Form.Get("redirect_uri") != TestRedirectURI) {
			t.Error("missing PKCE binding")
		}
		if calls == 2 && r.Form.Get("refresh_token") != "refresh-fixture" {
			t.Error("refresh token missing")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"access_token":"access-fixture","refresh_token":"refresh-fixture","token_type":"Bearer","expires_in":14400}`)), Header: make(http.Header)}, nil
	})}
	tokens, err := Exchange(context.Background(), client, TestClientID, a, "code-fixture")
	if err != nil {
		t.Fatal(err)
	}
	if time.Until(tokens.ExpiresAt) < 3*time.Hour {
		t.Fatal("expiry not recorded")
	}
	if _, err := Refresh(context.Background(), client, TestClientID, tokens); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatal("exchange or refresh missing")
	}
}

func TestBearerOnlyReachesExactWikiAPI(t *testing.T) {
	endpoint := "https://en.wikipedia.org/w/api.php"
	for _, destination := range []string{endpoint + "?action=query", "https://en.wikipedia.org/other", "https://evil.example/w/api.php", "https://en.wikipedia.org.evil.example/w/api.php", "http://en.wikipedia.org/w/api.php", "https://upload.wikimedia.org/file.png"} {
		transport := BearerTransport{Endpoint: endpoint, Token: "fixture", Base: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			want := ""
			if strings.HasPrefix(destination, endpoint+"?") {
				want = "Bearer fixture"
			}
			if got := r.Header.Get("Authorization"); got != want {
				t.Errorf("credential destination violation for %s", destination)
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		})}
		req, _ := http.NewRequest("GET", destination, nil)
		req.Header.Set("Authorization", "Bearer inherited")
		response, err := transport.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if req.Header.Get("Authorization") != "Bearer inherited" {
			t.Fatal("original request mutated")
		}
	}
}

func TestOAuthErrorsDoNotExposeResponseSecrets(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 400, Body: io.NopCloser(strings.NewReader(`{"error_description":"secret-fixture"}`))}, nil
	})}
	_, err := tokenRequest(context.Background(), client, "https://example.org", url.Values{})
	if err == nil || strings.Contains(fmt.Sprint(err), "secret-fixture") {
		t.Fatal("server error was exposed")
	}
}

func TestWikimediaRealmIsExactAndSharedLoginOnly(t *testing.T) {
	for _, host := range []string{"en.wikipedia.org", "commons.wikimedia.org", "www.wikidata.org", "meta.wikimedia.org", "www.mediawiki.org"} {
		if !IsWikimedia("https://" + host + "/w/api.php") {
			t.Errorf("missing shared-login wiki %s", host)
		}
	}
	for _, endpoint := range []string{"https://wikibase.world/w/api.php", "https://en.wikipedia.org.evil.example/w/api.php", "http://en.wikipedia.org/w/api.php", "https://en.wikipedia.org:444/w/api.php", "https://en.wikipedia.org/other/api.php", "https://office.wikimedia.org/w/api.php"} {
		if IsWikimedia(endpoint) {
			t.Errorf("untrusted realm member %s", endpoint)
		}
	}
	for _, raw := range []string{"https://user:pass@example.org/w/api.php", "https://example.org/w/api.php?redirect=x", "https://example.org/w/api.php#fragment", "http://example.org/w/api.php"} {
		if _, err := Endpoint(raw); err == nil {
			t.Fatal("unsafe endpoint accepted")
		}
	}
}
