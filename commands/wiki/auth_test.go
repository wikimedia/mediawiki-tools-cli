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

func isolateWikiAccounts(t *testing.T) {
	t.Helper()
	keyring.MockInit()
	if err := config.PutDiskConfig(koanf.New(".")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = config.PutDiskConfig(koanf.New(".")); savedWikiAccount = nil })
}

func seedWikiOAuth(t *testing.T, name string) config.WikiAccount {
	t.Helper()
	a := config.WikiAccount{Realm: "wikimedia", Name: name, Username: "Fixture", Method: "oauth", Endpoint: wikiauth.WikimediaAuthority, Wiki: wikiauth.WikimediaAuthority + "/w/api.php", ClientID: wikiauth.TestClientID, RedirectURI: wikiauth.TestRedirectURI}
	tokens := wikiauth.Tokens{AccessToken: "access-fixture", RefreshToken: "refresh-fixture", TokenType: "Bearer", ExpiresAt: time.Now().Add(time.Hour)}
	encoded, _ := json.Marshal(tokens)
	if err := a.Credential().SaveWithProvider(string(encoded), false, a.Metadata(), "keyring"); err != nil {
		t.Fatal(err)
	}
	return a
}

type wikiRoundTripFunc func(*http.Request) (*http.Response, error)

func (f wikiRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSavedOAuthAccountSelectionAndIsolation(t *testing.T) {
	isolateWikiAccounts(t)
	seedWikiOAuth(t, "personal")
	cmd := NewWikiCmd()
	cmd.SetArgs([]string{"page", "move", "From", "To", "--wiki", "commons.wikimedia.org", "--dry-run"})
	// Prepare directly: a dry run must select identity without opening a keyring.
	wiki = "commons.wikimedia.org"
	if err := prepareWikiAccount(cmd); err != nil {
		t.Fatal(err)
	}
	if savedWikiAccount == nil || wikiUser != "Fixture" {
		t.Fatal("Wikimedia account did not span wikis")
	}
	wiki = "https://wikibase.world/w/api.php"
	wikiAccount = "wikimedia/personal"
	wikiUser = ""
	if err := prepareWikiAccount(cmd); err == nil {
		t.Fatal("credential accepted for unrelated wiki")
	}
	wiki = "https://commons.wikimedia.org/w/api.php"
	wikiAccount = "wikimedia/personal"
	wikiUser = "SomeoneElse"
	if err := prepareWikiAccount(cmd); err == nil {
		t.Fatal("account allowed with overridden username")
	}
	data, _ := os.ReadFile(config.Path())
	if bytes.Contains(data, []byte("access-fixture")) || bytes.Contains(data, []byte("refresh-fixture")) {
		t.Fatal("OAuth tokens saved in ordinary config")
	}
}

func TestSavedOAuthRefreshPersistsAndStatusUsesNewToken(t *testing.T) {
	isolateWikiAccounts(t)
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
	cmd.SetArgs([]string{"auth", "status", "--account", "wikimedia/personal", "--refresh"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
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

func TestWikiAccountLogoutAndCredentialMapping(t *testing.T) {
	isolateWikiAccounts(t)
	a := seedWikiOAuth(t, "personal")
	credential, err := config.CredentialForKey(a.Credential().Key)
	if err != nil || credential != a.Credential() {
		t.Fatal("wiki token not supported by credential management")
	}
	if _, err := config.CredentialForKey("wiki.accounts.wikimedia.personal.password"); err == nil {
		t.Fatal("wrong authentication method accepted")
	}
	cmd := NewWikiCmd()
	cmd.SetOut(io.Discard)
	cmd.SetArgs([]string{"auth", "logout", "--account", "wikimedia/personal"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := config.LoadWikiAccount("wikimedia", "personal"); err == nil {
		t.Fatal("logout retained account")
	}
	if config.HasCredential(a.Credential().Key) {
		t.Fatal("logout retained credential reference")
	}
}

func TestBotPasswordLoginStoresOnlyReferenceAndSupportsCustomWiki(t *testing.T) {
	isolateWikiAccounts(t)
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
	cmd.SetArgs([]string{"auth", "login", "--realm", "local", "--account", "personal", "--wiki", server.URL + "/w/api.php", "--user", "Fixture@app", "--method", "bot-password", "--password-stdin", "--provider", "keyring"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	a, err := config.LoadWikiAccount("local", "personal")
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
			if !strings.HasPrefix(line, wikiauth.WikimediaAuthority+"/w/rest.php/oauth2/authorize?") {
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
			isolateWikiAccounts(t)
			if locked {
				keyring.MockInitWithError(errors.New("locked"))
			}
			previous := http.DefaultTransport
			defer func() { http.DefaultTransport = previous }()
			http.DefaultTransport = wikiRoundTripFunc(func(r *http.Request) (*http.Response, error) {
				body := `{"query":{"userinfo":{"id":123,"name":"Fixture"}}}`
				if strings.HasSuffix(r.URL.Path, "access_token") {
					_ = r.ParseForm()
					if r.Form.Has("client_secret") || r.Form.Get("code_verifier") == "" || r.Form.Get("code") != "code-fixture" {
						t.Error("PKCE exchange missing or client secret sent")
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
			cmd.SetArgs([]string{"auth", "login", "--realm", "wikimedia", "--account", "personal", "--manual", "--no-browser", "--provider", "keyring"})
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
			a, err := config.LoadWikiAccount("wikimedia", "personal")
			if locked {
				if err == nil {
					t.Fatal("failed save retained account")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			value, err := a.Credential().Read()
			if err != nil || !strings.Contains(value, "refresh-fixture") {
				t.Fatal("refresh token not in secure provider")
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
	a, err := wikiauth.Begin(wikiauth.TestClientID, redirect)
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
