package gerrit

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/knadh/koanf"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func setupAuthTest(t *testing.T) *bytes.Buffer {
	t.Helper()
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
	config.State()
	if err := config.PutDiskConfig(koanf.New(".")); err != nil {
		t.Fatal(err)
	}
	keyring.MockInit()
	log := &bytes.Buffer{}
	oldOut, oldLevel := logrus.StandardLogger().Out, logrus.GetLevel()
	logrus.SetOutput(log)
	logrus.SetLevel(logrus.TraceLevel)
	oldTransport := http.DefaultTransport
	http.DefaultTransport = transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.UserAgent() != cli.HTTPUserAgent {
			t.Errorf("unexpected User-Agent: %q", r.UserAgent())
		}
		user, password, ok := r.BasicAuth()
		if !ok || user != "fixture" || password != "fixture-secret" {
			t.Errorf("unexpected HTTP authentication")
		}
		if r.URL.Host != "gerrit.wikimedia.org" {
			t.Errorf("unexpected host: %s", r.URL.Host)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(")]}'\n\"fixture\"")), Request: r}, nil
	})
	t.Cleanup(func() {
		http.DefaultTransport = oldTransport
		logrus.SetOutput(oldOut)
		logrus.SetLevel(oldLevel)
		config.ReApplyKoanfConf(koanf.New("."))
	})
	return log
}

func TestLoginStdinStoresOnlyReferenceAndDoesNotLogPassword(t *testing.T) {
	log := setupAuthTest(t)
	out := &bytes.Buffer{}
	cmd := NewGerritCmd()
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetIn(strings.NewReader("fixture-secret\n"))
	if err := config.PutKeyValueOnDisk("secrets.provider", "keepassxc"); err != nil {
		t.Fatal(err)
	}
	cmd.SetArgs([]string{"auth", "login", "--username", "fixture", "--password-stdin", "--provider", "keyring"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(config.Path())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data)+out.String()+log.String(), "fixture-secret") {
		t.Fatal("credential leaked to disk or output")
	}
	cred, err := config.CredentialForKey("gerrit.password")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := cred.Read(); err != nil || got != "fixture-secret" {
		t.Fatalf("cannot read saved credential: %v", err)
	}
	logout := NewGerritCmd()
	logout.SetArgs([]string{"auth", "logout"})
	logout.SetOut(out)
	if err := logout.Execute(); err != nil {
		t.Fatal(err)
	}
	if config.HasCredential("gerrit.password") {
		t.Fatal("logout left saved credential")
	}
}

func TestStdinPasswordForOneCommandIsNotSaved(t *testing.T) {
	setupAuthTest(t)
	cmd := NewGerritCmd()
	cmd.SetIn(strings.NewReader("fixture-secret\n"))
	cmd.SetArgs([]string{"auth", "status", "--username", "fixture", "--password-stdin"})
	cmd.SetOut(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if config.HasCredential("gerrit.password") {
		t.Fatal("one-off password was persisted")
	}
}

func TestLocalGerritCommandDoesNotUnlockKeyring(t *testing.T) {
	setupAuthTest(t)
	cred := config.Credential{Key: "gerrit.password", Service: "gerrit", Endpoint: "https://gerrit.wikimedia.org/r", Account: "fixture"}
	if err := cred.SaveWithMetadata("fixture-secret", false, map[string]string{"gerrit.username": "fixture"}); err != nil {
		t.Fatal(err)
	}
	keyring.MockInitWithError(errors.New("locked"))
	cmd := NewGerritCmd()
	leaf, _, err := cmd.Find([]string{"dotgitreview", "host"})
	if err != nil {
		t.Fatal(err)
	}
	ran := false
	leaf.Run = func(_ *cobra.Command, _ []string) { ran = true }
	cmd.SetArgs([]string{"dotgitreview", "host"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !ran {
		t.Fatal("local command did not run")
	}
}
