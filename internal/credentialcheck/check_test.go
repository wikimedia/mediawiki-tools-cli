package credentialcheck

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiauth"
)

func TestServiceChecksAuthenticateAndReportOnlyStatus(t *testing.T) {
	for _, service := range []string{"phabricator", "gerrit", "cloudvps"} {
		for _, scenario := range []string{"valid", "invalid", "denied", "unavailable", "wrong-account"} {
			t.Run(service+"/"+scenario, func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if !strings.HasPrefix(r.UserAgent(), cli.HTTPUserAgent) {
						t.Error("CLI identity missing")
					}
					if scenario == "invalid" {
						w.WriteHeader(401)
						fmt.Fprint(w, "private-fixture")
						return
					}
					if scenario == "denied" {
						w.WriteHeader(403)
						return
					}
					if scenario == "unavailable" {
						w.WriteHeader(503)
						return
					}
					actual := "Fixture"
					project := "fixture"
					if scenario == "wrong-account" {
						actual = "Other"
						project = "other"
					}
					switch service {
					case "phabricator":
						if r.Method != "POST" || r.URL.Path != "/api/user.whoami" {
							t.Error("wrong Conduit endpoint")
						}
						_ = r.ParseForm()
						if !strings.Contains(r.Form.Get("params"), `"token":"private-fixture"`) {
							t.Error("token missing")
						}
						fmt.Fprintf(w, `{"result":{"userName":%q},"error_code":null}`, actual)
					case "gerrit":
						user, password, ok := r.BasicAuth()
						if !ok || user != "Fixture" || password != "private-fixture" || r.URL.Path != "/a/accounts/self/detail" {
							t.Error("invalid Gerrit authentication")
						}
						fmt.Fprintf(w, ")]}'\n{\"username\":%q}", actual)
					case "cloudvps":
						var payload map[string]any
						if json.NewDecoder(r.Body).Decode(&payload) != nil || r.URL.Path != "/auth/tokens" {
							t.Error("wrong Keystone request")
						}
						identity := payload["auth"].(map[string]any)["identity"].(map[string]any)
						credential := identity["application_credential"].(map[string]any)
						if credential["id"] != "id-fixture" || credential["secret"] != "private-fixture" {
							t.Error("application credential missing")
						}
						w.WriteHeader(201)
						fmt.Fprintf(w, `{"token":{"user":{"name":%q},"project":{"name":%q}}}`, actual, project)
					}
				}))
				defer server.Close()
				account := "Fixture"
				if service == "cloudvps" {
					account = "fixture:id-fixture"
				}
				result := Check(t.Context(), Request{Service: service, Endpoint: server.URL, Account: account, Value: "private-fixture"})
				if result.Status != scenario {
					t.Fatalf("status = %s, want %s", result.Status, scenario)
				}
				if strings.Contains(fmt.Sprint(result), "private-fixture") {
					t.Fatal("secret exposed")
				}
			})
		}
	}
}

func TestUsernameCaseComparisonIsServiceSpecific(t *testing.T) {
	for _, service := range []string{"phabricator", "gerrit"} {
		t.Run(service, func(t *testing.T) {
			for _, actual := range []string{"Addshore", "Addbot"} {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if service == "phabricator" {
						fmt.Fprintf(w, `{"result":{"userName":%q},"error_code":null}`, actual)
					} else {
						fmt.Fprintf(w, `{"username":%q}`, actual)
					}
				}))
				result := Check(t.Context(), Request{Service: service, Endpoint: server.URL, Account: "addshore", Value: "private-fixture"})
				server.Close()
				want := "wrong-account"
				if service == "phabricator" && actual == "Addshore" {
					want = "valid"
				}
				if result.Status != want || result.Identity != actual {
					t.Fatalf("%s: got %+v, want %s with identity %s", service, result, want, actual)
				}
			}
		})
	}
}

func TestCheckDoesNotForwardSecretsOnRedirect(t *testing.T) {
	targetCalls := 0
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { targetCalls++ }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 307) }))
	defer server.Close()
	result := Check(t.Context(), Request{Service: "phabricator", Endpoint: server.URL, Account: "Fixture", Value: "private-fixture"})
	if result.Status != "unavailable" || targetCalls != 0 {
		t.Fatal("redirect followed with credential")
	}
}

func TestPhabricatorConduitRejectionDoesNotExposeErrorInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"result":null,"error_code":"ERR-INVALID-AUTH","error_info":"private-fixture"}`)
	}))
	defer server.Close()
	result := Check(t.Context(), Request{Service: "phabricator", Endpoint: server.URL, Account: "Fixture", Value: "private-fixture"})
	if result.Status != "invalid" || strings.Contains(fmt.Sprint(result), "private-fixture") {
		t.Fatal("rejection mishandled")
	}
}

func TestWikiOAuthCheckUsesTokenWithoutRefreshing(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer private-fixture" || r.FormValue("meta") != "userinfo" {
			t.Error("wrong OAuth identity request")
		}
		fmt.Fprint(w, `{"query":{"userinfo":{"id":1,"name":"Fixture"}}}`)
	}))
	defer server.Close()
	for _, expired := range []bool{false, true} {
		expiry := time.Now().Add(time.Hour)
		want := "valid"
		if expired {
			expiry = time.Now().Add(-time.Hour)
			want = "expired"
		}
		bundle, _ := json.Marshal(wikiauth.Tokens{AccessToken: "private-fixture", RefreshToken: "refresh-private-fixture", TokenType: "Bearer", ExpiresAt: expiry})
		result := Check(t.Context(), Request{Service: "wiki", Endpoint: server.URL, Account: "Fixture", Value: string(bundle), WikiMethod: "oauth", WikiEndpoint: server.URL + "/w/api.php"})
		if result.Status != want {
			t.Fatalf("status = %s, want %s", result.Status, want)
		}
	}
	if calls != 1 {
		t.Fatal("expired token refreshed or unexpected request made")
	}
}

func TestChecksRespectCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	result := Check(ctx, Request{Service: "phabricator", Endpoint: "https://example.org", Account: "Fixture", Value: "private-fixture"})
	if result.Status != "unavailable" {
		t.Fatal("cancelled check not inconclusive")
	}
}

func TestWikiBotPasswordChecksLoginAndPreservesSession(t *testing.T) {
	for _, valid := range []bool{true, false} {
		t.Run(fmt.Sprint(valid), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = r.ParseForm()
				switch {
				case r.Form.Get("type") == "login":
					fmt.Fprint(w, `{"query":{"tokens":{"logintoken":"fixture-login-token"}}}`)
				case r.Form.Get("action") == "login":
					if r.Form.Get("lgname") != "Fixture@cli" || r.Form.Get("lgpassword") != "private-fixture" || r.Form.Get("lgtoken") != "fixture-login-token" {
						t.Error("wrong bot login parameters")
					}
					if !valid {
						fmt.Fprint(w, `{"login":{"result":"Failed","reason":"private-fixture"}}`)
						return
					}
					http.SetCookie(w, &http.Cookie{Name: "fixture-session", Value: "active", Path: "/"})
					fmt.Fprint(w, `{"login":{"result":"Success"}}`)
				default:
					if cookie, err := r.Cookie("fixture-session"); err != nil || cookie.Value != "active" {
						t.Error("authenticated session cookie was not retained")
					}
					fmt.Fprint(w, `{"query":{"userinfo":{"id":1,"name":"Fixture"}}}`)
				}
			}))
			defer server.Close()
			result := Check(t.Context(), Request{Service: "wiki", Endpoint: server.URL, Account: "Fixture@cli", Value: "private-fixture", WikiMethod: "bot-password", WikiEndpoint: server.URL + "/w/api.php"})
			want := "valid"
			if !valid {
				want = "invalid"
			}
			if result.Status != want {
				t.Fatalf("status = %s, want %s", result.Status, want)
			}
			if strings.Contains(fmt.Sprint(result), "private-fixture") {
				t.Fatal("API login error leaked credential")
			}
		})
	}
}
