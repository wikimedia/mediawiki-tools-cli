package enterprise

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	s := httptest.NewServer(handler)
	t.Cleanup(s.Close)
	c := NewClient()
	c.APIURL, c.AuthURL, c.RealtimeURL = s.URL, s.URL, s.URL
	c.Token = func(context.Context, string) (string, error) { return "access-secret", nil }
	return c
}

func TestLoginRefreshRevoke(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.Header.Get("Authorization") != "" {
			t.Error("unexpected auth request")
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		switch r.URL.Path {
		case "/v1/login":
			if body["username"] != "fixture" || body["password"] != "password-secret" {
				t.Error("bad login payload")
			}
			fmt.Fprint(w, `{"access_token":"initial","refresh_token":"refresh-secret","expires_in":86400}`)
		case "/v1/token-refresh":
			if body["username"] != "fixture" || body["refresh_token"] != "refresh-secret" {
				t.Error("bad refresh payload")
			}
			fmt.Fprint(w, `{"access_token":"refreshed","expires_in":86400}`)
		case "/v1/token-revoke":
			if body["refresh_token"] != "refresh-secret" {
				t.Error("bad revoke payload")
			}
			w.WriteHeader(204)
		default:
			t.Error("unexpected path", r.URL.Path)
		}
	})
	tokens, err := c.Login(t.Context(), "fixture", "password-secret")
	if err != nil {
		t.Fatal(err)
	}
	if time.Until(tokens.ExpiresAt) < 23*time.Hour {
		t.Fatal("bad expiry")
	}
	tokens, err = c.Refresh(t.Context(), "fixture", tokens)
	if err != nil {
		t.Fatal(err)
	}
	if tokens.RefreshToken != "refresh-secret" || tokens.AccessToken != "refreshed" {
		t.Fatal("refresh lost token state")
	}
	if err := c.Revoke(t.Context(), tokens); err != nil {
		t.Fatal(err)
	}
}

func TestUnauthorizedRetriesOnceAndErrorsAreRedacted(t *testing.T) {
	requests := 0
	c := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(401)
		fmt.Fprint(w, "private-server-secret")
	})
	var rejected []string
	c.Token = func(_ context.Context, old string) (string, error) {
		rejected = append(rejected, old)
		return "access-secret", nil
	}
	_, err := c.JSON(t.Context(), "/v2/projects", nil)
	var status *HTTPError
	if !errors.As(err, &status) || status.Status != 401 || requests != 2 || len(rejected) != 2 || rejected[1] != "access-secret" {
		t.Fatalf("bad retry: %v %d %v", err, requests, rejected)
	}
	if strings.Contains(err.Error(), "secret") {
		t.Fatal("error leaked server body")
	}
}

func TestJSONRetainsNumbersAndDoesNotFollowRedirects(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			w.Header().Set("Location", "/leak")
			w.WriteHeader(307)
			return
		}
		if r.URL.Path == "/leak" {
			t.Error("followed data redirect")
		}
		fmt.Fprint(w, `[{"identifier":9007199254740993}]`)
	})
	value, err := c.JSON(t.Context(), "/v2/projects", nil)
	if err != nil {
		t.Fatal(err)
	}
	if value.([]any)[0].(map[string]any)["identifier"].(json.Number).String() != "9007199254740993" {
		t.Fatal("identifier lost precision")
	}
	if _, err := c.JSON(t.Context(), "/redirect", nil); err == nil {
		t.Fatal("redirect accepted")
	}
}

func TestTokenResponseValidationAndCancellation(t *testing.T) {
	for _, body := range []string{`{}`, `{"access_token":"secret","expires_in":86400}`, `{"access_token":"secret","refresh_token":"refresh","expires_in":-1}`} {
		c := testClient(t, func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, body) })
		if _, err := c.Login(t.Context(), "user", "password"); err == nil {
			t.Fatal("accepted incomplete token response")
		}
	}
	c := testClient(t, func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, `{}`) })
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := c.JSON(ctx, "/v2/projects", nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation lost", err)
	}
}
