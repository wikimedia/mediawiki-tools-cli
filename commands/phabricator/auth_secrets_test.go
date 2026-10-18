package phabricator

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/knadh/koanf"
	"github.com/zalando/go-keyring"
	mwconfig "gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

type authTransport func(*http.Request) (*http.Response, error)

func (f authTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func isolatedAuthConfig(t *testing.T) {
	t.Helper()
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
	mwconfig.State()
	if err := mwconfig.PutDiskConfig(koanf.New(".")); err != nil {
		t.Fatal(err)
	}
	keyring.MockInit()
	t.Cleanup(func() { mwconfig.ReApplyKoanfConf(koanf.New(".")) })
}

func TestLoginIgnoresProjectDestinationAndLogoutDisablesLegacy(t *testing.T) {
	isolatedAuthConfig(t)
	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(old) })
	legacy := "[main]\ndefault=wikimedia\n[wikimedia]\nurl=https://untrusted.example\nkey=legacy-secret\nusername=fixture\n"
	if err := os.WriteFile(filepath.Join(dir, "phab.cfg"), []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	oldTransport := http.DefaultTransport
	http.DefaultTransport = authTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "phabricator.wikimedia.org" {
			t.Errorf("new token sent to project-selected host: %s", r.URL.Host)
		}
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		if !strings.Contains(r.Form.Get("params"), "fixture-secret") {
			t.Error("missing test token")
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"result":{"userName":"fixture"}}`)), Request: r}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = oldTransport })
	out := &bytes.Buffer{}
	cmd := NewPhabricatorCmd()
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetIn(strings.NewReader("fixture-secret\n"))
	if err := mwconfig.PutKeyValueOnDisk("secrets.provider", "keepassxc"); err != nil {
		t.Fatal(err)
	}
	cmd.SetArgs([]string{"auth", "login", "--username", "fixture", "--token-stdin", "--provider", "keyring"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(mwconfig.Path())
	if strings.Contains(string(data)+out.String(), "fixture-secret") {
		t.Fatal("token leaked")
	}
	cfg, err := loadConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Key != "fixture-secret" || cfg.URL != defaultPhabricatorURL {
		t.Fatal("legacy project config shadowed secure login")
	}
	logout := NewPhabricatorCmd()
	logout.SetOut(out)
	logout.SetArgs([]string{"auth", "logout"})
	if err := logout.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := loadConfig(""); err == nil {
		t.Fatal("logout fell back to legacy credential")
	}
	unchanged, _ := os.ReadFile(filepath.Join(dir, "phab.cfg"))
	if string(unchanged) != legacy {
		t.Fatal("logout modified external legacy file")
	}
}

func TestConduitDoesNotForwardTokenOnRedirect(t *testing.T) {
	calls := 0
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(200) }))
	defer destination.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, 307) }))
	defer redirect.Close()
	client := newConduitClient(&PhabConfig{URL: redirect.URL, Key: "fixture-secret"})
	if _, err := client.post("user.whoami", map[string]interface{}{}); err == nil {
		t.Fatal("redirect unexpectedly succeeded")
	}
	if calls != 0 {
		t.Fatal("forwarded token to redirect destination")
	}
}
