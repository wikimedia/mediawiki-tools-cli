package gitlab

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/knadh/koanf"
	"github.com/zalando/go-keyring"
	"gitlab.com/gitlab-org/cli/mwcli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func isolateGitlabProfiles(t *testing.T) string {
	t.Helper()
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
	t.Setenv("GLAB_CONFIG_DIR", t.TempDir())
	for _, key := range []string{profileEnvironment, "GITLAB_TOKEN", "OAUTH_TOKEN", "GITLAB_ACCESS_TOKEN", "CI_JOB_TOKEN", "GITLAB_API_HOST", "GITLAB_REPO", "GITLAB_HOST", "GL_HOST"} {
		t.Setenv(key, "")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	if err := config.PutDiskConfig(koanf.New(".")); err != nil {
		t.Fatal(err)
	}
	previous := cli.Opts
	cli.Opts.NoInteraction = true
	keyring.MockInit()
	t.Cleanup(func() { cli.Opts = previous; config.ReApplyKoanfConf(koanf.New(".")) })
	path := filepath.Join(os.Getenv("GLAB_CONFIG_DIR"), "config.yml")
	if err := os.WriteFile(path, []byte("hosts:\n  gitlab.wikimedia.org:\n    token: legacy-private-fixture\n    user: Legacy\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func runProfileCommand(input string, args ...string) (string, error) {
	return runProfileCommandWithClient(nil, input, args...)
}

func runProfileCommandWithClient(client *http.Client, input string, args ...string) (string, error) {
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	streams := &mwcli.IOStreams{In: io.NopCloser(strings.NewReader(input)), StdOut: out, StdErr: errOut}
	cmd := newCommand(mwcli.WithIOStreams(streams), mwcli.WithHTTPClient(client))
	cmd.SetIn(strings.NewReader(input))
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String() + errOut.String(), err
}

func savedGitlabProfile(t *testing.T, name, user, token string) {
	t.Helper()
	c := config.Credential{Key: "gitlab.profiles." + name + ".token", Service: "gitlab", Endpoint: gitlabEndpoint, Account: user}
	prefix := "gitlab.profiles." + name + "."
	if err := c.SaveWithProvider(token, false, map[string]string{prefix + "username": user, prefix + "url": gitlabEndpoint}, "keyring"); err != nil {
		t.Fatal(err)
	}
}

func TestGitlabProfileLoginImportAndManagement(t *testing.T) {
	path := isolateGitlabProfiles(t)
	original, _ := os.ReadFile(path)
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	username := "Addshore"
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != gitlabEndpoint+"/api/v4/user" || r.Header.Get("PRIVATE-TOKEN") == "" {
			t.Fatal("incorrect identity validation request")
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"username":"` + username + `"}`)), Request: r}, nil
	})
	if out, err := runProfileCommand("private-fixture", "auth", "login", "--token-stdin", "--provider", "keyring"); err != nil || !strings.Contains(out, "saved profile addshore") {
		t.Fatal(out, err)
	}
	username = "Addbot"
	if out, err := runProfileCommand("bot-private-fixture", "auth", "login", "--profile", "bot", "--token-stdin", "--provider", "keyring"); err != nil || strings.Contains(out, "private-fixture") {
		t.Fatal(out, err)
	}
	if gitlabProfiles.Default() != "addshore" {
		t.Fatal("second login switched default")
	}
	if _, err := runProfileCommand("", "auth", "default", "--profile", "bot"); err != nil {
		t.Fatal(err)
	}
	if out, err := runProfileCommand("", "auth", "status"); err != nil || !strings.Contains(out, "Addbot") {
		t.Fatal(out, err)
	}
	username = "Other"
	if out, err := runProfileCommand("", "auth", "status"); err == nil || !strings.Contains(err.Error(), "Other") || strings.Contains(out, "bot-private-fixture") {
		t.Fatal("wrong-account result missing", out, err)
	}
	username = "Imported"
	t.Setenv("GITLAB_TOKEN", "environment-private-fixture")
	seenToken := ""
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		seenToken = r.Header.Get("PRIVATE-TOKEN")
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"username":"Imported"}`)), Request: r}, nil
	})
	if _, err := runProfileCommand("", "auth", "login", "--from-glab", "--profile", "imported", "--provider", "keyring"); err != nil {
		t.Fatal(err)
	}
	if seenToken != "legacy-private-fixture" {
		t.Fatal("import used environment or active-profile token")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(original, after) {
		t.Fatal("import changed glab configuration")
	}
	entries := config.SecretInventory()
	if len(entries) != 3 {
		t.Fatalf("inventory has %d entries", len(entries))
	}
	for _, entry := range entries {
		if entry.Service != "gitlab" || entry.Provider != "keyring" {
			t.Fatal("incorrect inventory", entry)
		}
	}
	keyring.MockInitWithError(errors.New("locked"))
	if out, err := runProfileCommand("", "auth", "profiles"); err != nil || !strings.Contains(out, "bot (default)") {
		t.Fatal("metadata listing read secrets", out, err)
	}
	keyring.MockInit()
	// Recreate only the profile being removed; the other references remain distinct.
	savedGitlabProfile(t, "bot", "Addbot", "bot-private-fixture")
	if _, err := runProfileCommand("", "auth", "remove", "--profile", "bot"); err != nil {
		t.Fatal(err)
	}
	if gitlabProfiles.Default() != "" || config.State().OnDiskKoanf.Exists("gitlab.profiles.bot") {
		t.Fatal("profile/default retained")
	}
	if !config.State().OnDiskKoanf.Exists("gitlab.profiles.addshore") {
		t.Fatal("another profile removed")
	}
}

func TestGitlabProfilesOverrideUpstreamCredentialsAndRestrictHosts(t *testing.T) {
	isolateGitlabProfiles(t)
	t.Chdir(t.TempDir())
	t.Setenv("GITLAB_TOKEN", "wrong-environment-fixture")
	t.Setenv("CI_JOB_TOKEN", "wrong-job-fixture")
	t.Setenv("GITLAB_API_HOST", "unrelated.example")
	savedGitlabProfile(t, "personal", "Addshore", "selected-private-fixture")
	if err := config.PutKeyValueOnDisk("gitlab.default_profile", "personal"); err != nil {
		t.Fatal(err)
	}
	calls := 0
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Host != "gitlab.wikimedia.org" || r.Header.Get("PRIVATE-TOKEN") != "selected-private-fixture" || r.Header.Get("JOB-TOKEN") != "" {
			t.Fatal("wrong API identity or host")
		}
		header := make(http.Header)
		header.Set("Content-Type", "application/json")
		body := `{"username":"Addshore"}`
		if r.URL.Path == "/api/v4/projects" {
			header.Set("Link", `<https://unrelated.example/api/v4/projects?page=2>; rel="next"`)
			body = `[]`
		}
		if r.URL.Path == "/api/v4/redirect" {
			header.Set("Location", "https://unrelated.example/")
			return &http.Response{StatusCode: 302, Header: header, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
		}
		return &http.Response{StatusCode: 200, Header: header, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})}
	if _, err := runProfileCommandWithClient(client, "", "api", "user"); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"api", "user", "--hostname", "unrelated.example"},
		{"api", "https://unrelated.example/api/v4/user"},
		{"api", "http://gitlab.wikimedia.org/api/v4/user"},
		{"api", "projects", "--paginate"},
		{"api", "redirect"},
	} {
		before := calls
		if _, err := runProfileCommandWithClient(client, "", args...); err == nil {
			t.Fatal("unsafe request succeeded", args)
		}
		if strings.Contains(strings.Join(args, " "), "unrelated.example") && calls != before {
			t.Fatal("foreign endpoint reached transport")
		}
	}
	if os.Getenv("GITLAB_TOKEN") != "wrong-environment-fixture" || os.Getenv("CI_JOB_TOKEN") != "wrong-job-fixture" {
		t.Fatal("environment not restored")
	}
	keyring.MockInitWithError(errors.New("locked"))
	before := calls
	if _, err := runProfileCommandWithClient(client, "", "api", "user"); err == nil || calls != before {
		t.Fatal("locked profile fell back", err)
	}
	if _, err := runProfileCommandWithClient(client, "", "api", "user", "--profile", "missing"); err == nil || calls != before {
		t.Fatal("missing profile fell back", err)
	}
}

func TestGitlabCredentialHelperPinsProfileAndHost(t *testing.T) {
	isolateGitlabProfiles(t)
	savedGitlabProfile(t, "personal", "Addshore", "helper-private-fixture")
	if err := config.PutKeyValueOnDisk("gitlab.default_profile", "personal"); err != nil {
		t.Fatal(err)
	}
	out, err := runProfileCommand("protocol=https\nhost=gitlab.wikimedia.org\n\n", "auth", "git-credential", "get")
	if err != nil || !strings.Contains(out, "password=helper-private-fixture") {
		t.Fatal("helper did not use profile", err)
	}
	for _, input := range []string{"protocol=http\nhost=gitlab.wikimedia.org\n\n", "protocol=https\nhost=unrelated.example\n\n", "protocol=https\nhost=gitlab.wikimedia.org\nusername=Other\n\n"} {
		out, err := runProfileCommand(input, "auth", "git-credential", "get")
		if err == nil || strings.Contains(out, "helper-private-fixture") {
			t.Fatal("helper disclosed credential for mismatched request")
		}
	}
	for _, op := range []string{"store", "erase"} {
		if _, err := runProfileCommand("", "auth", "git-credential", op); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := runProfileCommand("", "config", "get", "token"); err == nil {
		t.Fatal("upstream config exposed profile token")
	}
	if _, err := runProfileCommand("", "config", "set", "oauth_token", "other-fixture"); err == nil {
		t.Fatal("upstream config wrote token")
	}
}

func TestGitlabProfileConfigAdapterDoesNotPersistTokens(t *testing.T) {
	path := isolateGitlabProfiles(t)
	before, _ := os.ReadFile(path)
	factory, err := mwcli.NewProfileFactory(glabVersion(), cli.HTTPUserAgent(), func() (*mwcli.ProfileCredentials, error) {
		return &mwcli.ProfileCredentials{Host: "gitlab.wikimedia.org", Username: "Addshore", Token: func() (string, error) { return "adapter-private-fixture", nil }}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg := factory.Config()
	if token, err := cfg.Get("gitlab.wikimedia.org", "oauth_token"); err != nil || token != "adapter-private-fixture" {
		t.Fatal("adapter not used", err)
	}
	if token, err := cfg.Get("unrelated.example", "token"); err == nil || token != "" {
		t.Fatal("foreign host received credential")
	}
	if err := cfg.Set("gitlab.wikimedia.org", "token", "adapter-private-fixture"); err == nil {
		t.Fatal("profile token persisted")
	}
	if err := cfg.Write(); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if strings.Contains(string(after), "adapter-private-fixture") || !bytes.Contains(before, []byte("legacy-private-fixture")) {
		t.Fatal("credential persistence changed")
	}
}

func TestGitlabLoginFailureDoesNotSaveProfile(t *testing.T) {
	isolateGitlabProfiles(t)
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	calls := 0
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 401, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("private-fixture")), Request: r}, nil
	})
	out, err := runProfileCommand("private-fixture", "auth", "login", "--token-stdin", "--provider", "keyring")
	if err == nil || strings.Contains(out+err.Error(), "private-fixture") || len(gitlabProfiles.Names()) != 0 || gitlabProfiles.Default() != "" {
		t.Fatal("failed validation exposed or saved credentials")
	}
	before := calls
	if _, err := runProfileCommand("private-fixture", "auth", "login", "--token-stdin", "--profile", "invalid.name"); err == nil || calls != before {
		t.Fatal("invalid name reached service")
	}
}

func TestGitlabChildGitEnvironmentIsScopedAndRestored(t *testing.T) {
	isolateGitlabProfiles(t)
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "core.editor")
	t.Setenv("GIT_CONFIG_VALUE_0", "original-editor")
	t.Setenv("GITLAB_TOKEN", "environment-private-fixture")
	profile := &mwcli.ProfileCredentials{Host: "gitlab.wikimedia.org", Username: "Addshore", Token: func() (string, error) { return "selected-private-fixture", nil }}
	restore, err := profileGitEnvironment("bot", profile)
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	if os.Getenv("GIT_CONFIG_COUNT") != "4" || os.Getenv("GIT_CONFIG_VALUE_0") != "original-editor" || os.Getenv("GIT_CONFIG_VALUE_1") != "" {
		t.Fatal("inherited configuration or helper reset missing")
	}
	if os.Getenv("GITLAB_TOKEN") != "" || os.Getenv(profileEnvironment) != "bot" || !strings.Contains(os.Getenv("GIT_CONFIG_VALUE_2"), "git-credential") {
		t.Fatal("child identity not scoped")
	}
	for _, entry := range os.Environ() {
		if strings.Contains(entry, "selected-private-fixture") {
			t.Fatal("profile token entered environment")
		}
	}
	restore()
	if os.Getenv("GIT_CONFIG_COUNT") != "1" || os.Getenv("GITLAB_TOKEN") != "environment-private-fixture" {
		t.Fatal("environment not restored")
	}
}

func TestGitlabAPIProfileHostIsIndependentOfCheckout(t *testing.T) {
	isolateGitlabProfiles(t)
	t.Chdir(t.TempDir())
	runGit(t, "init", "--quiet")
	runGit(t, "remote", "add", "origin", "https://gitlab.wikimedia.org/repos/releng/cli.git")
	credential := config.Credential{Key: "gitlab.profiles.custom.token", Service: "gitlab", Endpoint: "https://gitlab.example", Account: "Addshore"}
	if err := credential.SaveWithProvider("custom-private-fixture", false, map[string]string{"gitlab.profiles.custom.username": "Addshore", "gitlab.profiles.custom.url": "https://gitlab.example"}, "keyring"); err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "gitlab.example" || r.Header.Get("PRIVATE-TOKEN") != "custom-private-fixture" {
			t.Fatal("API inherited checkout host or credentials")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"username":"Addshore"}`)), Request: r}, nil
	})}
	if _, err := runProfileCommandWithClient(client, "", "api", "user", "--profile", "custom"); err != nil {
		t.Fatal(err)
	}
	if _, err := runProfileCommandWithClient(client, "", "release", "list", "--profile", "custom"); err == nil {
		t.Fatal("repository command ignored mismatched project host")
	}
}
