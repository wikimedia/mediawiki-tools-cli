package gerrit

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/knadh/koanf"
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
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(log, &slog.HandlerOptions{Level: slog.LevelDebug})))
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
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string([]byte{41, 93, 125, 39, 10}) + `{"username":"fixture","name":"Fixture"}`)), Request: r}, nil
	})
	t.Cleanup(func() {
		http.DefaultTransport = oldTransport
		slog.SetDefault(oldLogger)
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
	cred, err := config.CredentialForKey("gerrit.profiles.fixture.password")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := cred.Read(); err != nil || got != "fixture-secret" {
		t.Fatalf("cannot read saved credential: %v", err)
	}
	logout := NewGerritCmd()
	logout.SetArgs([]string{"auth", "remove"})
	logout.SetOut(out)
	if err := logout.Execute(); err != nil {
		t.Fatal(err)
	}
	if config.HasCredential("gerrit.profiles.fixture.password") {
		t.Fatal("remove left saved credential")
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
	leaf.RunE = func(_ *cobra.Command, _ []string) error { ran = true; return nil }
	cmd.SetArgs([]string{"dotgitreview", "host"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !ran {
		t.Fatal("local command did not run")
	}
}

func TestAuthProfilesDoesNotReadCredentials(t *testing.T) {
	setupAuthTest(t)
	if err := config.PutKeyValueOnDisk("gerrit.profiles.fixture.username", "fixture"); err != nil {
		t.Fatal(err)
	}
	credential := config.Credential{Key: "gerrit.profiles.fixture.password", Service: "gerrit", Endpoint: "https://gerrit.wikimedia.org/r", Account: "fixture"}
	if err := credential.SaveWithProvider("fixture-secret", false, nil, "keyring"); err != nil {
		t.Fatal(err)
	}
	keyring.MockInitWithError(errors.New("locked"))
	for _, name := range []string{"profiles", "list"} {
		out := &bytes.Buffer{}
		cmd := NewGerritCmd()
		cmd.SetOut(out)
		cmd.SetArgs([]string{"auth", name})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "fixture") || !strings.Contains(out.String(), "Saved Gerrit profiles") {
			t.Fatal(out.String())
		}
	}
}
