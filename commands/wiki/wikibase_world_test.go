package wiki

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiauth"
)

func TestWikibaseWorldOAuthPresetAndRefresh(t *testing.T) {
	isolateWikiProfiles(t)
	seedWikiOAuth(t, "personal")
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	refreshFails := false
	exchanges, refreshes := 0, 0
	http.DefaultTransport = wikiRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "wikibase.world" {
			t.Fatal("wikibase.world credentials sent to another host")
		}
		body := `{"query":{"userinfo":{"id":123,"name":"Addshore"}}}`
		code := 200
		if strings.HasSuffix(r.URL.Path, "access_token") {
			_ = r.ParseForm()
			if r.Form.Get("client_id") != wikiauth.WikibaseWorldClientID || r.Form.Has("client_secret") {
				t.Fatal("wrong client registration or confidential authentication")
			}
			if r.Form.Get("grant_type") == "authorization_code" {
				exchanges++
			} else {
				refreshes++
			}
			body = `{"access_token":"world-access-fixture","refresh_token":"world-refresh-fixture","token_type":"Bearer","expires_in":14400}`
			if refreshFails {
				code = 401
				body = `{"error":"invalid_client","error_description":"world-refresh-fixture"}`
			}
		} else if r.URL.Path != "/w/api.php" || r.Header.Get("Authorization") != "Bearer world-access-fixture" {
			t.Fatal("identity not checked against exact wiki API")
		}
		return &http.Response{StatusCode: code, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	var out bytes.Buffer
	cmd := NewWikiCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(&oauthCallbackReader{output: &out})
	cmd.SetArgs([]string{"auth", "login", "--profile", "wikibase-world/addshore", "--manual", "--no-browser", "--provider", "keyring", "--scope", "basic,editpage"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(out.String(), err)
	}
	a, err := config.LoadWikiProfile(wikiauth.WikibaseWorldRealm, "addshore")
	if err != nil {
		t.Fatal(err)
	}
	if a.ClientID != wikiauth.WikibaseWorldClientID || a.Wiki != wikiauth.WikibaseWorldAPI || a.Endpoint != wikiauth.WikibaseWorldAPI || a.RequestedScope != "basic editpage" {
		t.Fatal("preset metadata incorrect", a)
	}
	if a.Credential().Key != "wiki.profiles.wikibase-world.addshore.token" || a.Allows("https://meta.wikimedia.org/w/api.php") {
		t.Fatal("realm/key binding incorrect")
	}
	if config.State().OnDiskKoanf.String("wiki.default_profile") != "wikimedia/personal" {
		t.Fatal("another realm's default changed")
	}
	before, err := a.Credential().Read()
	if err != nil {
		t.Fatal(err)
	}
	refreshFails = true
	text, err := runWikiProfile(t, []string{"auth", "status", "--profile", "wikibase-world/addshore", "--refresh"}, "")
	if err == nil || !strings.Contains(err.Error(), "--replace") || strings.Contains(text+err.Error(), "world-refresh-fixture") {
		t.Fatal("refresh failure needs safe reauthorization guidance", text, err)
	}
	after, err := a.Credential().Read()
	if err != nil || before != after {
		t.Fatal("failed refresh overwrote saved tokens")
	}
	refreshFails = false
	if _, err := runWikiProfile(t, []string{"auth", "status", "--profile", "wikibase-world/addshore", "--refresh"}, ""); err != nil {
		t.Fatal(err)
	}
	if exchanges != 1 || refreshes != 2 {
		t.Fatal("exchange/refresh not exercised", exchanges, refreshes)
	}
	keyring.MockInitWithError(errors.New("locked"))
	if _, err := runWikiProfile(t, []string{"auth", "status", "--profile", "wikibase-world/addshore", "--wiki", "https://en.wikipedia.org/w/api.php"}, ""); err == nil || !strings.Contains(err.Error(), "scope") {
		t.Fatal("foreign wiki reached credential store", err)
	}
	if strings.Contains(out.String(), "world-access-fixture") || strings.Contains(out.String(), "world-refresh-fixture") {
		t.Fatal("tokens printed")
	}
}

func TestBuiltinWikibaseWorldCompletionAndScope(t *testing.T) {
	isolateWikiProfiles(t)
	realm, err := realmForWiki(wikiauth.WikibaseWorldAPI)
	if err != nil || realm != wikiauth.WikibaseWorldRealm {
		t.Fatal("built-in realm not detected", realm, err)
	}
	cmd := NewWikiCmd()
	values, _ := completeWikiSites(cmd, nil, "https://wikibase.world")
	if len(values) != 1 || values[0] != wikiauth.WikibaseWorldAPI {
		t.Fatal("built-in site not completed", values)
	}
	for _, args := range [][]string{
		{"realm", "add", "wikibase-world", "--endpoint", wikiauth.WikibaseWorldAPI},
		{"auth", "login", "--profile", "wikibase-world/other", "--site", "https://evil.example/w/api.php"},
		{"auth", "login", "--profile", "custom/other", "--wiki", "https://custom.example/w/api.php"},
	} {
		if _, err := runWikiProfile(t, args, ""); err == nil {
			t.Fatal("unsafe or unconfigured realm accepted", args)
		}
	}
}

func TestWikibaseWorldStillSupportsPasswordsAndBotPasswords(t *testing.T) {
	for _, user := range []string{"Addshore", "Addshore@cli"} {
		t.Run(user, func(t *testing.T) {
			isolateWikiProfiles(t)
			previous := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = previous })
			logins := 0
			http.DefaultTransport = wikiRoundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host != "wikibase.world" || r.URL.Path != "/w/api.php" || r.Header.Get("Authorization") != "" {
					t.Fatal("password login used OAuth or another wiki")
				}
				_ = r.ParseForm()
				body := `{"query":{"tokens":{"logintoken":"login-fixture"},"userinfo":{"id":42,"name":"Addshore"}}}`
				if r.Form.Get("action") == "login" {
					logins++
					if r.Form.Get("lgname") != user || r.Form.Get("lgpassword") != "world-password-fixture" {
						t.Fatal("incorrect password login")
					}
					body = `{"login":{"result":"Success","lgusername":"Addshore","lguserid":42}}`
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
			})
			text, err := runWikiProfile(t, []string{"auth", "login", "--profile", "wikibase-world/password", "--username", user, "--password-stdin", "--provider", "keyring"}, "world-password-fixture\n")
			if err != nil || strings.Contains(text, "world-password-fixture") {
				t.Fatal(text, err)
			}
			profile, err := config.LoadWikiProfile(wikiauth.WikibaseWorldRealm, "password")
			if err != nil || profile.Method != "password" || profile.Username != user || profile.Identity != "Addshore" || profile.Credential().Key != "wiki.profiles.wikibase-world.password.password" || logins != 1 {
				t.Fatal("password profile not preserved", profile, err)
			}
		})
	}
}
