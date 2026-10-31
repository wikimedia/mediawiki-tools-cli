package gerrit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func setupProfileAccounts(t *testing.T) {
	t.Helper()
	setupAuthTest(t)
	http.DefaultTransport = transportFunc(func(r *http.Request) (*http.Response, error) {
		user, pass, ok := r.BasicAuth()
		if !ok || pass != user+"-private-fixture" {
			return nil, fmt.Errorf("incorrect test credentials")
		}
		if r.URL.Path != "/r/a/accounts/self" {
			t.Fatalf("unexpected identity endpoint: %s", r.URL.Path)
		}
		data, _ := json.Marshal(map[string]string{"username": user, "name": "Display " + user})
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(")]}'\n" + string(data))), Request: r}, nil
	})
}

func executeProfile(t *testing.T, args ...string) (string, error) {
	t.Helper()
	out := &bytes.Buffer{}
	cmd := NewGerritCmd()
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func loginProfile(t *testing.T, user string, args ...string) {
	t.Helper()
	base := []string{"auth", "login", "--username", user, "--password", user + "-private-fixture", "--provider", "keyring"}
	if _, err := executeProfile(t, append(base, args...)...); err != nil {
		t.Fatal(err)
	}
}

func TestGerritProfileIsolationDefaultAndRemoval(t *testing.T) {
	setupProfileAccounts(t)
	loginProfile(t, "personal")
	loginProfile(t, "bot")
	if gerritProfiles.Default() != "personal" {
		t.Fatal("later login changed default")
	}
	for _, name := range []string{"personal", "bot"} {
		out, err := executeProfile(t, "auth", "status", "--profile", name)
		if err != nil || !strings.Contains(out, "Authenticated as "+name) {
			t.Fatalf("status %s: %s %v", name, out, err)
		}
		if strings.Contains(out, "private-fixture") {
			t.Fatal("password leaked")
		}
	}
	if _, err := executeProfile(t, "auth", "default", "--profile", "bot"); err != nil {
		t.Fatal(err)
	}
	out, err := executeProfile(t, "auth", "status")
	if err != nil || !strings.Contains(out, "Authenticated as bot") {
		t.Fatal("default not selected", out, err)
	}
	if _, err := executeProfile(t, "auth", "remove", "--profile", "personal"); err != nil {
		t.Fatal(err)
	}
	if config.HasCredential("gerrit.profiles.personal.password") || config.State().OnDiskKoanf.Exists("gerrit.profiles.personal") {
		t.Fatal("removed profile retained")
	}
	if _, err := executeProfile(t, "auth", "status", "--profile", "personal"); err == nil {
		t.Fatal("unknown profile used another account")
	}
	if _, err := executeProfile(t, "auth", "status", "--profile", "bot"); err != nil {
		t.Fatal("removal affected bot", err)
	}
	if _, err := executeProfile(t, "auth", "remove"); err != nil {
		t.Fatal(err)
	}
	if gerritProfiles.Default() != "" {
		t.Fatal("removed default retained")
	}
}

func TestGerritProfileDefaultAndCompletionDoNotUnlock(t *testing.T) {
	setupProfileAccounts(t)
	loginProfile(t, "personal")
	loginProfile(t, "bot")
	keyring.MockInitWithError(errors.New("locked"))
	for _, name := range []string{"profiles", "list"} {
		if _, err := executeProfile(t, "auth", name); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := executeProfile(t, "auth", "default", "--profile", "bot"); err != nil {
		t.Fatal(err)
	}
	values, _ := gerritProfiles.Complete(nil, nil, "p")
	if len(values) != 1 || values[0] != "personal" {
		t.Fatal(values)
	}
	if config.State().Effective.Gerrit.Username != "bot" {
		t.Fatal("default public identity not available to clone commands")
	}
	cmd := NewGerritCmd()
	leaf, _, err := cmd.Find([]string{"ssh"})
	if err != nil {
		t.Fatal(err)
	}
	ran := false
	leaf.RunE = func(_ *cobra.Command, _ []string) error {
		ran = true
		args := sshGerritCommand([]string{"version"}).Args
		if !strings.Contains(strings.Join(args, " "), "-l personal") {
			t.Fatal("SSH ignored selected identity", args)
		}
		return nil
	}
	cmd.SetArgs([]string{"ssh", "--profile", "personal", "version"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !ran {
		t.Fatal("SSH profile command did not run")
	}
}

func TestGerritExplicitLoginDefaultAndInvalidProfiles(t *testing.T) {
	setupProfileAccounts(t)
	loginProfile(t, "personal")
	loginProfile(t, "bot", "--profile", "automation", "--default")
	if gerritProfiles.Default() != "automation" {
		t.Fatal("explicit default ignored")
	}
	loginProfile(t, "personal", "--default=false")
	if gerritProfiles.Default() != "automation" {
		t.Fatal("explicit keep default ignored")
	}
	if _, err := executeProfile(t, "auth", "login", "--username", "personal", "--password", "personal-private-fixture", "--profile", "invalid.name", "--provider", "keyring"); err == nil {
		t.Fatal("invalid profile accepted")
	}
}

func TestGerritProfileSelectionAppliesToGeneratedCommands(t *testing.T) {
	setupProfileAccounts(t)
	loginProfile(t, "personal")
	loginProfile(t, "bot")
	cmd := NewGerritCmd()
	leaf, _, err := cmd.Find([]string{"changes", "list"})
	if err != nil {
		t.Fatal(err)
	}
	ran := false
	leaf.Run = nil
	leaf.RunE = func(_ *cobra.Command, _ []string) error {
		ran = true
		if gerritUsername != "bot" || gerritPassword != "bot-private-fixture" {
			t.Fatal("generated command ignored profile")
		}
		return nil
	}
	cmd.SetArgs([]string{"changes", "list", "--profile", "bot"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !ran {
		t.Fatal("generated command did not run")
	}
}

func TestGerritFailedSaveKeepsDefaultAndProfileRegistry(t *testing.T) {
	setupProfileAccounts(t)
	loginProfile(t, "personal")
	keyring.MockInitWithError(errors.New("locked"))
	_, err := executeProfile(t, "auth", "login", "--username", "bot", "--password", "bot-private-fixture", "--provider", "keyring", "--default")
	if err == nil {
		t.Fatal("failed keyring write accepted")
	}
	if gerritProfiles.Default() != "personal" || config.State().OnDiskKoanf.Exists("gerrit.profiles.bot") {
		t.Fatal("failed save changed default or profile metadata")
	}
}
