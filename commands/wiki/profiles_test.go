package wiki

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiauth"
)

func passwordWikiFixture(t *testing.T, user string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		if r.Form.Get("action") == "login" {
			if r.Form.Get("lgpassword") != "private-password-fixture" {
				t.Error("unexpected password")
			}
			fmt.Fprintf(w, `{"login":{"result":"Success","lgusername":%q,"lguserid":42}}`, user)
			return
		}
		fmt.Fprintf(w, `{"query":{"tokens":{"logintoken":"login-fixture"},"userinfo":{"id":42,"name":%q}}}`, user)
	}))
	t.Cleanup(server.Close)
	return server
}

func runWikiProfile(t *testing.T, args []string, input string) (string, error) {
	t.Helper()
	out := &bytes.Buffer{}
	cmd := NewWikiCmd()
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetIn(strings.NewReader(input))
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestWikiPasswordProfileArbitrarySiteAndScope(t *testing.T) {
	isolateWikiProfiles(t)
	server := passwordWikiFixture(t, "Addshore")
	api := server.URL + "/w/api.php"
	other := server.URL + "/other/api.php"
	out, err := runWikiProfile(t, []string{"auth", "login", "--wiki", api, "--username", "Addshore", "--password-stdin", "--provider", "keyring", "--site", other}, "private-password-fixture\n")
	if err != nil {
		t.Fatal(out, err)
	}
	realm, _, _ := strings.Cut(config.State().OnDiskKoanf.String("wiki.default_profile"), "/")
	profile, err := config.LoadWikiProfile(realm, "addshore")
	if err != nil {
		t.Fatal(err)
	}
	if profile.Method != "password" || profile.Identity != "Addshore" || !profile.Allows(api) || !profile.Allows(other) || profile.Allows(server.URL+"/untrusted/api.php") {
		t.Fatal("incorrect profile identity/scope", profile)
	}
	if strings.Contains(out, "private-password-fixture") {
		t.Fatal("password leaked")
	}
	keyring.MockInitWithError(errors.New("locked"))
	// Scope validation must reject this destination before attempting a keyring read.
	_, err = runWikiProfile(t, []string{"auth", "status", "--profile", realm + "/addshore", "--wiki", server.URL + "/untrusted/api.php"}, "")
	if err == nil || !strings.Contains(err.Error(), "scope") {
		t.Fatalf("out-of-scope destination was not rejected first: %v", err)
	}
	for _, args := range [][]string{{"auth", "profiles"}, {"auth", "default", "--profile", realm + "/addshore"}} {
		if _, err := runWikiProfile(t, args, ""); err != nil {
			t.Fatal("metadata command unlocked credentials", err)
		}
	}
}

func TestWikiProfilesPreserveGlobalAndRealmDefaults(t *testing.T) {
	isolateWikiProfiles(t)
	first := seedWikiOAuth(t, "personal")
	initial := "wikimedia/" + first.Name
	server := passwordWikiFixture(t, "Addbot")
	api := server.URL + "/w/api.php"
	args := []string{"auth", "login", "--wiki", api, "--realm", "custom", "--username", "Addbot@app", "--password-stdin", "--provider", "keyring"}
	if _, err := runWikiProfile(t, args, "private-password-fixture\n"); err != nil {
		t.Fatal(err)
	}
	if got := config.State().OnDiskKoanf.String("wiki.default_profile"); got != initial {
		t.Fatal("later login replaced global default", got)
	}
	if config.State().OnDiskKoanf.String("wiki.realms.custom.default_profile") != "addbot" {
		t.Fatal("first profile was not realm default")
	}
	if _, err := runWikiProfile(t, []string{"auth", "default", "--profile", "custom/addbot"}, ""); err != nil {
		t.Fatal(err)
	}
	if config.State().OnDiskKoanf.String("wiki.default_profile") != "custom/addbot" {
		t.Fatal("explicit default failed")
	}
	if _, err := runWikiProfile(t, []string{"auth", "remove", "--profile", "custom/addbot"}, ""); err != nil {
		t.Fatal(err)
	}
	if config.State().OnDiskKoanf.Exists("wiki.profiles.custom.addbot") || config.State().OnDiskKoanf.Exists("wiki.default_profile") {
		t.Fatal("removed profile/default retained")
	}
	if _, err := config.LoadWikiProfile("wikimedia", "personal"); err != nil {
		t.Fatal("removal affected another realm", err)
	}
}

func TestWikiBuiltInRealmAndRestrictedCompletion(t *testing.T) {
	isolateWikiProfiles(t)
	a := seedWikiOAuth(t, "personal")
	if !a.Allows("https://en.wikipedia.org/w/api.php") || !a.Allows("https://commons.wikimedia.org/w/api.php") || a.Allows("https://wikibase.world/w/api.php") {
		t.Fatal("Wikimedia membership incorrect")
	}
	disk := config.State().OnDiskKoanf.Copy()
	if err := disk.Set("wiki.profiles.wikimedia.personal.sites", []string{a.Wiki}); err != nil {
		t.Fatal(err)
	}
	if err := config.PutDiskConfig(disk); err != nil {
		t.Fatal(err)
	}
	keyring.MockInitWithError(io.ErrUnexpectedEOF)
	cmd := NewWikiCmd()
	_ = cmd.PersistentFlags().Set("profile", "wikimedia/personal")
	values, _ := completeWikiSites(cmd, nil, "https://en.wikipedia.")
	if len(values) != 0 {
		t.Fatal("site completion ignored profile scope", values)
	}
}

func TestWikiProfileAliasesAndInvalidNames(t *testing.T) {
	isolateWikiProfiles(t)
	seedWikiOAuth(t, "personal")
	cmd := NewWikiCmd()
	if !cmd.PersistentFlags().Lookup("account").Hidden || !cmd.PersistentFlags().Lookup("user").Hidden {
		t.Fatal("legacy aliases are visible")
	}
	_, err := runWikiProfile(t, []string{"auth", "profiles", "--account", "wikimedia/personal", "--profile", "wikimedia/personal"}, "")
	if err == nil {
		t.Fatal("both profile flags accepted")
	}
	_, err = runWikiProfile(t, []string{"auth", "login", "--profile", strings.Repeat("x", 65), "--method", "password", "--wiki", wikiauth.WikimediaAuthority + "/w/api.php", "--username", "Addshore", "--password-stdin", "--provider", "keyring"}, "private-password-fixture\n")
	if err == nil {
		t.Fatal("invalid profile name accepted")
	}
}

func TestWikiOAuthProfileNameScopeAndRefresh(t *testing.T) {
	isolateWikiProfiles(t)
	seedWikiOAuth(t, "personal")
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	http.DefaultTransport = wikiRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := `{"query":{"userinfo":{"id":123,"name":"Fixture"}}}`
		if strings.HasSuffix(r.URL.Path, "access_token") {
			_ = r.ParseForm()
			if r.Form.Get("grant_type") == "refresh_token" && r.Form.Get("scope") != "basic" {
				t.Error("refresh broadened requested scopes")
			}
			body = `{"access_token":"private-access-fixture","refresh_token":"private-refresh-fixture","token_type":"Bearer","expires_in":14400}`
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	var out bytes.Buffer
	cmd := NewWikiCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(&oauthCallbackReader{output: &out})
	api := "https://test.wikipedia.org/w/api.php"
	cmd.SetArgs([]string{"auth", "login", "--manual", "--no-browser", "--provider", "keyring", "--scope", "basic", "--site", api})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	a, err := config.LoadWikiProfile("wikimedia", "fixture")
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Sites) != 1 || a.Sites[0] != api || a.RequestedScope != "basic" || a.Allows("https://en.wikipedia.org/w/api.php") {
		t.Fatal("OAuth profile scope not saved", a)
	}
	if config.State().OnDiskKoanf.String("wiki.default_profile") != "wikimedia/personal" {
		t.Fatal("OAuth login replaced the existing default")
	}
	if _, err := runWikiProfile(t, []string{"auth", "status", "--profile", "wikimedia/fixture", "--refresh"}, ""); err != nil {
		t.Fatal(err)
	}
	a, err = config.LoadWikiProfile("wikimedia", "fixture")
	if err != nil || len(a.Sites) != 1 {
		t.Fatal("refresh lost profile site scope", err)
	}
	if strings.Contains(out.String(), "private-access-fixture") || strings.Contains(out.String(), "private-refresh-fixture") {
		t.Fatal("OAuth token leaked")
	}
}

func TestWikiExplicitDefaultFalseAndReplaceRetainsScope(t *testing.T) {
	isolateWikiProfiles(t)
	server := passwordWikiFixture(t, "Fixture")
	api := server.URL + "/w/api.php"
	args := []string{"auth", "login", "--wiki", api, "--realm", "local", "--profile", "personal", "--username", "Fixture@app", "--password-stdin", "--provider", "keyring", "--default=false"}
	if _, err := runWikiProfile(t, args, "private-password-fixture\n"); err != nil {
		t.Fatal(err)
	}
	disk := config.State().OnDiskKoanf
	if disk.Exists("wiki.default_profile") || disk.Exists("wiki.realms.local.default_profile") {
		t.Fatal("explicit --default=false assigned a default")
	}
	if _, err := runWikiProfile(t, append(args, "--replace"), "private-password-fixture\n"); err != nil {
		t.Fatal(err)
	}
	a, err := config.LoadWikiProfile("local", "personal")
	if err != nil || len(a.Sites) != 1 || a.Sites[0] != api {
		t.Fatal("reauthorization broadened/lost site scope", err)
	}
}
