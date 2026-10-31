package wiki

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/knadh/koanf"
	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiauth"
)

func TestMain(m *testing.M) {
	directory, err := os.MkdirTemp("", "wiki-command-tests-*")
	if err != nil {
		panic(err)
	}
	_ = os.Setenv("GITLAB_CI", "true")
	_ = os.Setenv("CI_PROJECT_DIR", directory)
	code := m.Run()
	_ = os.RemoveAll(directory)
	os.Exit(code)
}

func isolateWikiProfiles(t *testing.T) {
	t.Helper()
	keyring.MockInit()
	if err := config.PutDiskConfig(koanf.New(".")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = config.PutDiskConfig(koanf.New(".")); savedWikiProfile = nil })
}

func seedWikiOAuth(t *testing.T, name string) config.WikiProfile {
	t.Helper()
	a := config.WikiProfile{Realm: "wikimedia", Name: name, Username: "Fixture", Method: "oauth", Endpoint: wikiauth.WikimediaAuthority, Wiki: wikiauth.WikimediaAuthority + "/w/api.php", ClientID: wikiauth.DefaultClientID, RedirectURI: wikiauth.DefaultRedirectURI}
	tokens := wikiauth.Tokens{AccessToken: "access-fixture", RefreshToken: "refresh-fixture", TokenType: "Bearer", ExpiresAt: time.Now().Add(time.Hour)}
	metadata := a.Metadata()
	if config.State().OnDiskKoanf.String("wiki.default_profile") == "" {
		metadata["wiki.default_profile"] = a.Realm + "/" + a.Name
	}
	if config.State().OnDiskKoanf.String("wiki.realms."+a.Realm+".default_profile") == "" {
		metadata["wiki.realms."+a.Realm+".default_profile"] = a.Name
	}
	encoded, _ := json.Marshal(tokens)
	if err := a.Credential().SaveWithProvider(string(encoded), false, metadata, "keyring"); err != nil {
		t.Fatal(err)
	}
	return a
}

type wikiRoundTripFunc func(*http.Request) (*http.Response, error)

func (f wikiRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSavedOAuthAccountSelectionAndIsolation(t *testing.T) {
	isolateWikiProfiles(t)
	seedWikiOAuth(t, "personal")
	cmd := NewWikiCmd()
	cmd.SetArgs([]string{"page", "move", "From", "To", "--wiki", "commons.wikimedia.org", "--dry-run"})
	// Prepare directly: a dry run must select identity without opening a keyring.
	wiki = "commons.wikimedia.org"
	if err := prepareWikiProfile(cmd); err != nil {
		t.Fatal(err)
	}
	if savedWikiProfile == nil || wikiUser != "Fixture" {
		t.Fatal("Wikimedia account did not span wikis")
	}
	wiki = "https://wikibase.world/w/api.php"
	wikiProfile = "wikimedia/personal"
	wikiUser = ""
	if err := prepareWikiProfile(cmd); err == nil {
		t.Fatal("credential accepted for unrelated wiki")
	}
	wiki = "https://commons.wikimedia.org/w/api.php"
	wikiProfile = "wikimedia/personal"
	wikiUser = "SomeoneElse"
	if err := prepareWikiProfile(cmd); err == nil {
		t.Fatal("account allowed with overridden username")
	}
	data, _ := os.ReadFile(config.Path())
	if bytes.Contains(data, []byte("access-fixture")) || bytes.Contains(data, []byte("refresh-fixture")) {
		t.Fatal("OAuth tokens saved in ordinary config")
	}
}

func TestSavedOAuthRefreshPersistsAndStatusUsesNewToken(t *testing.T) {
	isolateWikiProfiles(t)
	a := seedWikiOAuth(t, "personal")
	previous := http.DefaultTransport
	defer func() { http.DefaultTransport = previous }()
	refreshes := 0
	http.DefaultTransport = wikiRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := `{"query":{"userinfo":{"id":123,"name":"Fixture"}}}`
		if strings.HasSuffix(r.URL.Path, "access_token") {
			refreshes++
			_ = r.ParseForm()
			if r.Form.Has("client_secret") || r.Form.Get("refresh_token") != "refresh-fixture" {
				t.Error("invalid public refresh")
			}
			body = `{"access_token":"new-access-fixture","refresh_token":"new-refresh-fixture","token_type":"Bearer","expires_in":14400}`
		} else if r.Header.Get("Authorization") != "Bearer new-access-fixture" {
			t.Error("status did not use saved refreshed token")
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	var output bytes.Buffer
	cmd := NewWikiCmd()
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"auth", "status", "--profile", "wikimedia/personal", "--refresh"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, stage := range []string{"Reading saved OAuth credentials", "Refreshing the OAuth token", "Saving refreshed OAuth credentials", "Checking the authenticated wiki identity"} {
		if !strings.Contains(output.String(), stage) {
			t.Errorf("refresh did not report stage %q", stage)
		}
	}
	if refreshes != 1 || !strings.Contains(output.String(), "Authenticated as Fixture") {
		t.Fatal("refresh or identity check failed")
	}
	value, err := a.Credential().Read()
	if err != nil || !strings.Contains(value, "new-refresh-fixture") {
		t.Fatal("rotated refresh token was not saved")
	}
	data, _ := os.ReadFile(config.Path())
	if strings.Contains(string(data)+output.String(), "new-refresh-fixture") {
		t.Fatal("tokens leaked")
	}
	// Another invocation must work solely from persisted credentials.
	cmd = NewWikiCmd()
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"auth", "status"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if refreshes != 1 {
		t.Fatal("fresh token unnecessarily refreshed")
	}
}

func TestWikiProfileLogoutAndCredentialMapping(t *testing.T) {
	isolateWikiProfiles(t)
	a := seedWikiOAuth(t, "personal")
	credential, err := config.CredentialForKey(a.Credential().Key)
	if err != nil || credential != a.Credential() {
		t.Fatal("wiki token not supported by credential management")
	}
	if _, err := config.CredentialForKey("wiki.profiles.wikimedia.personal.password"); err == nil {
		t.Fatal("wrong authentication method accepted")
	}
	cmd := NewWikiCmd()
	cmd.SetOut(io.Discard)
	cmd.SetArgs([]string{"auth", "remove", "--profile", "wikimedia/personal"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := config.LoadWikiProfile("wikimedia", "personal"); err == nil {
		t.Fatal("logout retained account")
	}
	if config.HasCredential(a.Credential().Key) {
		t.Fatal("logout retained credential reference")
	}
}

func TestBotPasswordLoginStoresOnlyReferenceAndSupportsCustomWiki(t *testing.T) {
	isolateWikiProfiles(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		if r.Form.Get("action") == "login" {
			if r.Form.Get("lgpassword") != "bot-fixture" {
				t.Error("incorrect password")
			}
			fmt.Fprint(w, `{"login":{"result":"Success","lgusername":"Fixture","lguserid":123}}`)
			return
		}
		fmt.Fprint(w, `{"query":{"tokens":{"logintoken":"login-fixture"},"userinfo":{"id":123,"name":"Fixture"}}}`)
	}))
	defer server.Close()
	var output bytes.Buffer
	cmd := NewWikiCmd()
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetIn(strings.NewReader("bot-fixture\n"))
	cmd.SetArgs([]string{"auth", "login", "--realm", "local", "--profile", "personal", "--wiki", server.URL + "/w/api.php", "--user", "Fixture@app", "--method", "bot-password", "--password-stdin", "--provider", "keyring"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	a, err := config.LoadWikiProfile("local", "personal")
	if err != nil {
		t.Fatal(err)
	}
	if !config.WikiRealmAllows("local", server.URL+"/w/api.php") {
		t.Fatal("custom wiki not bound to realm")
	}
	if value, err := a.Credential().Read(); err != nil || value != "bot-fixture" {
		t.Fatal("password not stored in provider")
	}
	data, _ := os.ReadFile(config.Path())
	if strings.Contains(string(data)+output.String(), "bot-fixture") {
		t.Fatal("password leaked")
	}
	if _, err := accountTokens(context.Background(), a, false); err == nil {
		t.Fatal("bot password accepted as OAuth bundle")
	}
}

type oauthCallbackReader struct {
	output *bytes.Buffer
	reader *strings.Reader
}

func (r *oauthCallbackReader) Read(p []byte) (int, error) {
	if r.reader == nil {
		for _, line := range strings.Split(r.output.String(), "\n") {
			if !strings.Contains(line, "/rest.php/oauth2/authorize?") {
				continue
			}
			u, err := url.Parse(line)
			if err != nil {
				return 0, err
			}
			q := u.Query()
			r.reader = strings.NewReader(q.Get("redirect_uri") + "?" + url.Values{"state": {q.Get("state")}, "code": {"code-fixture"}}.Encode() + "\n")
		}
		if r.reader == nil {
			return 0, fmt.Errorf("authorization URL was not printed")
		}
	}
	return r.reader.Read(p)
}

func TestOAuthLoginCommandSavesVerifiedTokensAndFailsClosed(t *testing.T) {
	for _, locked := range []bool{false, true} {
		t.Run(fmt.Sprint(locked), func(t *testing.T) {
			isolateWikiProfiles(t)
			if locked {
				keyring.MockInitWithError(errors.New("locked"))
			}
			previous := http.DefaultTransport
			defer func() { http.DefaultTransport = previous }()
			http.DefaultTransport = wikiRoundTripFunc(func(r *http.Request) (*http.Response, error) {
				body := `{"query":{"userinfo":{"id":123,"name":"Fixture"}}}`
				if strings.HasSuffix(r.URL.Path, "access_token") {
					_ = r.ParseForm()
					if r.Form.Has("client_secret") {
						t.Error("public client sent a client secret")
					}
					if r.Form.Get("grant_type") == "authorization_code" && (r.Form.Get("code_verifier") == "" || r.Form.Get("code") != "code-fixture") {
						t.Error("PKCE exchange missing")
					}
					if r.Form.Get("grant_type") == "refresh_token" && r.Form.Get("scope") != "basic" {
						t.Error("refresh broadened the explicitly scoped login")
					}
					body = `{"access_token":"access-fixture","refresh_token":"refresh-fixture","token_type":"Bearer","expires_in":14400}`
				} else if r.Header.Get("Authorization") != "Bearer access-fixture" {
					t.Error("identity not authenticated")
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
			})
			var output bytes.Buffer
			cmd := NewWikiCmd()
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			cmd.SetIn(&oauthCallbackReader{output: &output})
			cmd.SetArgs([]string{"auth", "login", "--realm", "wikimedia", "--profile", "personal", "--manual", "--no-browser", "--provider", "keyring", "--scope", "basic"})
			err := cmd.Execute()
			if locked && err == nil {
				t.Fatal("locked provider silently accepted")
			}
			if !locked && err != nil {
				t.Fatal(err)
			}
			data, _ := os.ReadFile(config.Path())
			if strings.Contains(string(data)+output.String(), "access-fixture") || strings.Contains(string(data)+output.String(), "refresh-fixture") {
				t.Fatal("OAuth tokens leaked")
			}
			a, err := config.LoadWikiProfile("wikimedia", "personal")
			if locked {
				if err == nil {
					t.Fatal("failed save retained account")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if a.RequestedScope != "basic" || !strings.Contains(output.String(), "scope=basic") {
				t.Fatal("explicit scope restriction was not requested or saved")
			}
			value, err := a.Credential().Read()
			if err != nil || !strings.Contains(value, "refresh-fixture") {
				t.Fatal("refresh token not in secure provider")
			}
			var tokens wikiauth.Tokens
			if json.Unmarshal([]byte(value), &tokens) != nil || tokens.RequestedScope != "basic" {
				t.Fatal("refresh scope restriction missing from secure token bundle")
			}
			status := NewWikiCmd()
			status.SetOut(&output)
			status.SetErr(&output)
			status.SetArgs([]string{"auth", "status", "--profile", "wikimedia/personal", "--refresh"})
			if err := status.Execute(); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(output.String(), "Requested OAuth scopes: basic") {
				t.Fatal("status did not label the requested scope restriction")
			}
		})
	}
}

func TestOAuthLoopbackListenerRejectsWrongStateAndCompletes(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	redirect := "http://" + listener.Addr().String() + "/auth/callback"
	_ = listener.Close()
	a, err := wikiauth.Begin(wikiauth.DefaultClientID, redirect)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := &cobra.Command{}
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	done := make(chan error, 1)
	go func() {
		var err error
		client := &http.Client{Timeout: time.Second}
		var response *http.Response
		for ctx.Err() == nil {
			response, err = client.Get(redirect + "?code=fixture&state=wrong")
			if err == nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if err != nil {
			done <- err
			return
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusBadRequest {
			done <- fmt.Errorf("wrong state accepted")
			return
		}
		response, err = client.Get(redirect + "?" + url.Values{"code": {"fixture"}, "state": {a.State}}.Encode())
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode != http.StatusOK {
				err = fmt.Errorf("valid callback rejected")
			}
		}
		done <- err
	}()
	callback, err := authorizeWiki(ctx, cmd, a, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if code, err := a.Code(callback); err != nil || code != "fixture" {
		t.Fatal("callback not returned")
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestOAuthReauthorizationRequiresExplicitReplacement(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(fmt.Sprint(replace), func(t *testing.T) {
			isolateWikiProfiles(t)
			original := seedWikiOAuth(t, "personal")
			previous := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = previous })
			calls := 0
			http.DefaultTransport = wikiRoundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				body := `{"query":{"userinfo":{"id":123,"name":"Fixture"}}}`
				if strings.HasSuffix(r.URL.Path, "access_token") {
					body = `{"access_token":"replacement-access","refresh_token":"replacement-refresh","token_type":"Bearer","expires_in":14400}`
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
			})
			var output bytes.Buffer
			cmd := NewWikiCmd()
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			cmd.SetIn(&oauthCallbackReader{output: &output})
			args := []string{"auth", "login", "--profile", "wikimedia/personal", "--provider", "keyring", "--manual", "--no-browser", "--scope", "basic"}
			if replace {
				args = append(args, "--replace")
			}
			cmd.SetArgs(args)
			err := cmd.Execute()
			if !replace {
				if err == nil || !strings.Contains(err.Error(), "--replace") || calls != 0 {
					t.Fatalf("existing login was not protected: calls=%d, err=%v", calls, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			value, err := original.Credential().Read()
			if err != nil || !strings.Contains(value, "replacement-refresh") {
				t.Fatal("reauthorization did not update the existing credential slot")
			}
			if strings.Contains(output.String(), "replacement-refresh") {
				t.Fatal("replacement token appeared in output")
			}
		})
	}
}
