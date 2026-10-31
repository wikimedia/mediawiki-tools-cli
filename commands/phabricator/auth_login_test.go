package phabricator

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	mwconfig "gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func mockLoginIdentity(t *testing.T) {
	t.Helper()
	old := http.DefaultTransport
	http.DefaultTransport = authTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "phabricator.wikimedia.org" || r.URL.Path != "/api/user.whoami" {
			t.Fatalf("unexpected request: %s", r.URL)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		username := "Addshore"
		if strings.Contains(r.Form.Get("params"), "bot-secret") {
			username = "Addbot"
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`{"result":{"userName":%q}}`, username))), Request: r}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = old })
}

func TestBrowserLoginAndExistingTokenProfiles(t *testing.T) {
	isolatedAuthConfig(t)
	mockLoginIdentity(t)
	opened, prompted := 0, 0
	profile := "personal"
	interaction := loginInteraction{
		canAsk: func() bool { return true },
		openURL: func(url string) error {
			opened++
			if url != defaultPhabricatorURL+"/conduit/login/" {
				t.Fatal(url)
			}
			return fmt.Errorf("browser unavailable")
		},
		readToken: func() (string, error) { prompted++; return "cli-personal-secret", nil },
	}
	out := &bytes.Buffer{}
	browserCmd := newPhabricatorAuthLoginCmdWithInteraction(&profile, interaction)
	browserCmd.SetOut(out)
	browserCmd.SetArgs([]string{"--provider", "keyring"})
	if err := browserCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if opened != 1 || prompted != 1 || !strings.Contains(out.String(), "open the URL above manually") {
		t.Fatal("browser flow or fallback missing", out.String())
	}
	personal, err := loadConfig("personal")
	if err != nil || personal.Username != "Addshore" || personal.Key != "cli-personal-secret" {
		t.Fatalf("personal: %+v, %v", personal, err)
	}
	botCmd := NewPhabricatorCmd()
	botCmd.SetOut(out)
	botCmd.SetIn(strings.NewReader("api-bot-secret\n"))
	botCmd.SetArgs([]string{"auth", "login", "--profile", "bot", "--method", "token", "--token-stdin", "--provider", "keyring"})
	if err := botCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	bot, err := loadConfig("bot")
	if err != nil || bot.Username != "Addbot" || bot.Key != "api-bot-secret" {
		t.Fatalf("bot: %+v, %v", bot, err)
	}
	personal, err = loadConfig("personal")
	if err != nil || personal.Key != "cli-personal-secret" {
		t.Fatal("bot login changed personal profile", err)
	}
	status := NewPhabricatorCmd()
	status.SetOut(out)
	status.SetArgs([]string{"auth", "status", "--profile", "personal"})
	if err := status.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Authenticated as Addshore") {
		t.Fatal(out.String())
	}
	for _, secret := range []string{"cli-personal-secret", "api-bot-secret"} {
		if strings.Contains(out.String(), secret) || strings.Contains(mwconfig.State().OnDiskKoanf.Sprint(), secret) {
			t.Fatal("secret leaked")
		}
	}
	remove := NewPhabricatorCmd()
	remove.SetOut(out)
	remove.SetArgs([]string{"auth", "remove", "--profile", "bot"})
	if err := remove.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := loadConfig("bot"); err == nil {
		t.Fatal("logged out bot accepted")
	}
	if _, err := loadConfig("personal"); err != nil {
		t.Fatal("bot remove changed personal", err)
	}
}

func TestLoginRejectsWrongAccountAndInvalidProfile(t *testing.T) {
	isolatedAuthConfig(t)
	mockLoginIdentity(t)
	for _, args := range [][]string{
		{"--profile", "wrong", "--username", "Addbot", "--token", "cli-personal-secret"},
		{"--profile", "bad.name", "--token", "cli-personal-secret"},
		{"--profile", "one", "--site", "two", "--token", "cli-personal-secret"},
		{"--method", "oauth", "--token", "cli-personal-secret"},
	} {
		cmd := NewPhabricatorCmd()
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(append([]string{"auth", "login", "--provider", "keyring"}, args...))
		if err := cmd.Execute(); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	if mwconfig.HasCredential("phabricator.profiles.wrong.key") {
		t.Fatal("saved mismatched identity")
	}
}

func TestProfileListAndCompletionDoNotReadCredentials(t *testing.T) {
	isolatedAuthConfig(t)
	for key, value := range map[string]string{"phabricator.profiles.personal.username": "Addshore", "phabricator.profiles.bot.username": "Addbot", "phabricator.default_profile": "personal"} {
		if err := mwconfig.PutKeyValueOnDisk(key, value); err != nil {
			t.Fatal(err)
		}
	}
	got, directive := phabProfiles.Complete(nil, nil, "p")
	if len(got) != 1 || got[0] != "personal" || directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatal(got, directive)
	}
	out := &bytes.Buffer{}
	cmd := NewPhabricatorCmd()
	cmd.SetOut(out)
	cmd.SetArgs([]string{"auth", "list"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "personal (default)") || !strings.Contains(out.String(), "Addbot") {
		t.Fatal(out.String())
	}
}

func TestUnknownProfileDoesNotReuseFlatCredential(t *testing.T) {
	isolatedAuthConfig(t)
	for key, value := range map[string]string{"phabricator.key": "legacy-secret", "phabricator.username": "Legacy"} {
		if err := mwconfig.PutKeyValueOnDisk(key, value); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := configFromMWCLI("unknown"); err == nil {
		t.Fatal("unknown profile reused the flat credential")
	}
}

func TestLoginPromptChoices(t *testing.T) {
	for _, test := range []struct {
		name    string
		args    []string
		wantURL string
	}{
		{name: "manual browser", args: []string{"--no-browser"}, wantURL: "/conduit/login/"},
		{name: "existing token", args: []string{"--method", "token"}, wantURL: "/settings/panel/apitokens/"},
	} {
		t.Run(test.name, func(t *testing.T) {
			isolatedAuthConfig(t)
			mockLoginIdentity(t)
			profile := "personal"
			cmd := newPhabricatorAuthLoginCmdWithInteraction(&profile, loginInteraction{
				canAsk:    func() bool { return true },
				openURL:   func(string) error { t.Fatal("unexpected browser launch"); return nil },
				readToken: func() (string, error) { return "cli-personal-secret", nil },
			})
			out := &bytes.Buffer{}
			cmd.SetOut(out)
			cmd.SetArgs(append(test.args, "--provider", "keyring"))
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), test.wantURL) {
				t.Fatal(out.String())
			}
		})
	}
}

func TestLoginDefaultProfileSelection(t *testing.T) {
	for _, test := range []struct {
		name        string
		current     string
		interactive bool
		answer      bool
		flag        string
		want        string
		wantPrompt  bool
	}{
		{name: "first login", want: "bot"},
		{name: "noninteractive preserves default", current: "personal", want: "personal"},
		{name: "decline switch", current: "personal", interactive: true, wantPrompt: true, want: "personal"},
		{name: "accept switch", current: "personal", interactive: true, answer: true, wantPrompt: true, want: "bot"},
		{name: "explicit switch", current: "personal", flag: "--default", want: "bot"},
		{name: "explicit keep", current: "personal", interactive: true, flag: "--default=false", want: "personal"},
		{name: "same profile", current: "bot", interactive: true, want: "bot"},
	} {
		t.Run(test.name, func(t *testing.T) {
			isolatedAuthConfig(t)
			mockLoginIdentity(t)
			if test.current != "" {
				key := "phabricator.default_profile"
				if err := mwconfig.PutKeyValueOnDisk(key, test.current); err != nil {
					t.Fatal(err)
				}
			}
			profile := "bot"
			prompted := false
			cmd := newPhabricatorAuthLoginCmdWithInteraction(&profile, loginInteraction{
				canAsk: func() bool { return test.interactive },
				confirmDefault: func(profile, current string) (bool, error) {
					prompted = true
					if profile != "bot" || current != test.current {
						t.Fatalf("unexpected prompt: %s %s", profile, current)
					}
					return test.answer, nil
				},
			})
			out := &bytes.Buffer{}
			cmd.SetOut(out)
			args := []string{"--token", "api-bot-secret", "--provider", "keyring"}
			if test.flag != "" {
				args = append(args, test.flag)
			}
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if got := phabProfiles.Default(); got != test.want {
				t.Fatalf("default=%q, want %q", got, test.want)
			}
			if prompted != test.wantPrompt {
				t.Fatalf("prompted=%v, want %v", prompted, test.wantPrompt)
			}
			cfg, err := loadConfig("bot")
			if err != nil || cfg.Username != "Addbot" {
				t.Fatalf("profile not saved: %+v %v", cfg, err)
			}
			if test.want == "bot" && mwconfig.State().OnDiskKoanf.String("phabricator.default_profile") != "bot" {
				t.Fatal("new default not saved under default_profile")
			}
		})
	}
}

func TestDefaultProfileSelection(t *testing.T) {
	isolatedAuthConfig(t)
	for key, value := range map[string]string{
		"phabricator.default_site":    "old",
		"phabricator.default_profile": "personal",
	} {
		if err := mwconfig.PutKeyValueOnDisk(key, value); err != nil {
			t.Fatal(err)
		}
	}
	if got := resolveAuthSite(""); got != "personal" {
		t.Fatal(got)
	}
	if got := resolveAuthSite("bot"); got != "bot" {
		t.Fatal(got)
	}
}

func TestLoginWithoutProfileUsesVerifiedUsername(t *testing.T) {
	isolatedAuthConfig(t)
	mockLoginIdentity(t)
	if err := mwconfig.PutKeyValueOnDisk("phabricator.default_profile", "personal"); err != nil {
		t.Fatal(err)
	}
	cmd := NewPhabricatorCmd()
	cmd.SetOut(io.Discard)
	cmd.SetArgs([]string{"auth", "login", "--token", "api-bot-secret", "--provider", "keyring"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig("addbot")
	if err != nil || cfg.Username != "Addbot" {
		t.Fatalf("username-derived profile: %+v %v", cfg, err)
	}
	if phabProfiles.Default() != "personal" {
		t.Fatal("login replaced existing default")
	}
}

func TestRemoveProfileDeletesMetadataAndDefault(t *testing.T) {
	for _, action := range []string{"remove"} {
		t.Run(action, func(t *testing.T) {
			isolatedAuthConfig(t)
			for key, value := range map[string]string{
				"phabricator.profiles.wikimedia.url":  defaultPhabricatorURL,
				"phabricator.profiles.other.username": "Other",
				"phabricator.default_profile":         "wikimedia",
			} {
				if err := mwconfig.PutKeyValueOnDisk(key, value); err != nil {
					t.Fatal(err)
				}
			}
			out := &bytes.Buffer{}
			cmd := NewPhabricatorCmd()
			cmd.SetOut(out)
			cmd.SetArgs([]string{"auth", action, "--profile", "wikimedia"})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			disk := mwconfig.State().OnDiskKoanf
			if disk.Exists("phabricator.profiles.wikimedia") || disk.Exists("phabricator.default_profile") {
				t.Fatal("removed profile metadata/default retained")
			}
			if !disk.Exists("phabricator.profiles.other") {
				t.Fatal("another profile was removed")
			}
			if !strings.Contains(out.String(), "credentials are not revoked") {
				t.Fatal("missing server token explanation")
			}
			profiles, _ := phabProfiles.Complete(nil, nil, "")
			if len(profiles) != 1 || profiles[0] != "other" {
				t.Fatal("removed profile still listed", profiles)
			}
		})
	}
}

func TestPhabricatorLogoutCommandIsNotAvailable(t *testing.T) {
	cmd := NewPhabricatorCmd()
	auth, _, err := cmd.Find([]string{"auth"})
	if err != nil {
		t.Fatal(err)
	}
	for _, child := range auth.Commands() {
		if child.Name() == "logout" {
			t.Fatal("logout command is still available")
		}
		for _, alias := range child.Aliases {
			if alias == "logout" {
				t.Fatal("logout alias is still available")
			}
		}
	}
}
