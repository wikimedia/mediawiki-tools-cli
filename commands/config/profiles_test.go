package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/knadh/koanf"
	"github.com/zalando/go-keyring"
	mwconfig "gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

type profileNoNetwork struct{ t *testing.T }

func (p profileNoNetwork) RoundTrip(*http.Request) (*http.Response, error) {
	p.t.Error("profile inventory contacted a service")
	return nil, errors.New("network disabled")
}

func TestConfigProfilesListsAllServicesWithoutReadingSecrets(t *testing.T) {
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
	if err := mwconfig.PutDiskConfig(koanf.New(".")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { mwconfig.ReApplyKoanfConf(koanf.New(".")) })
	keyring.MockInit()
	credential := mwconfig.Credential{Key: "gerrit.profiles.addshore.password", Service: "gerrit", Endpoint: "https://gerrit.wikimedia.org/r", Account: "Addshore"}
	if err := credential.SaveWithProvider("gerrit-private-fixture", false, map[string]string{"gerrit.profiles.addshore.username": "Addshore", "gerrit.default_profile": "addshore"}, "keyring"); err != nil {
		t.Fatal(err)
	}
	disk := mwconfig.State().OnDiskKoanf.Copy()
	for key, value := range map[string]string{
		"enterprise.profiles.bot.username": "EnterpriseBot", "enterprise.profiles.bot.token": "enterprise-private-fixture", "enterprise.default_profile": "bot",
		"gitlab.profiles.bot.username": "Addbot", "gitlab.profiles.bot.token": "gitlab-private-fixture", "gitlab.default_profile": "bot",
		"phabricator.profiles.bot.username": "Addbot", "phabricator.profiles.bot.key": "phab-private-fixture",
		"wiki.profiles.wikimedia.personal.username": "Addshore", "wiki.profiles.wikimedia.personal.method": "oauth", "wiki.profiles.wikimedia.personal.token": "oauth-private-fixture",
		"wiki.default_profile": "wikimedia/personal", "wiki.realms.wikimedia.default_profile": "personal",
		"wiki.profiles.independent.other.username": "Other", "wiki.profiles.independent.other.method": "password", "wiki.profiles.independent.other.password": "wiki-private-fixture",
		"cloud_vps.profiles.demo.id": "public-cloud-id", "cloud_vps.profiles.demo.secret": "cloud-private-fixture", "cloud_vps.default_profile": "demo", "cloud_vps.profiles.demo.project": "demo",
	} {
		if err := disk.Set(key, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := disk.Set("wiki.profiles.independent.other.sites", []string{"https://wikibase.world/w/api.php"}); err != nil {
		t.Fatal(err)
	}
	if err := mwconfig.PutDiskConfig(disk); err != nil {
		t.Fatal(err)
	}
	keyring.MockInitWithError(errors.New("locked"))
	previous := http.DefaultTransport
	http.DefaultTransport = profileNoNetwork{t: t}
	t.Cleanup(func() { http.DefaultTransport = previous })
	for _, args := range [][]string{{"profiles"}, {"profiles", "list", "--details"}, {"profiles", "--json"}, {"profiles", "--service", "phab", "--json"}, {"profiles", "--service", "enterprise", "--json"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			out := &bytes.Buffer{}
			cmd := Cmd()
			cmd.SetOut(out)
			cmd.SetErr(out)
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			for _, value := range []string{"gerrit-private-fixture", "phab-private-fixture", "oauth-private-fixture", "wiki-private-fixture", "cloud-private-fixture", "gitlab-private-fixture", "enterprise-private-fixture"} {
				if strings.Contains(out.String(), value) {
					t.Fatal("secret leaked")
				}
			}
			if strings.Contains(strings.Join(args, " "), "--json") {
				var profiles []mwconfig.ProfileInfo
				if err := json.Unmarshal(out.Bytes(), &profiles); err != nil {
					t.Fatal(err)
				}
				want := 7
				if strings.Contains(strings.Join(args, " "), "--service") {
					want = 1
				}
				if len(profiles) != want {
					t.Fatalf("got %d profiles, want %d", len(profiles), want)
				}
				if want == 7 {
					defaults := 0
					for _, profile := range profiles {
						if profile.Default {
							defaults++
						}
						if profile.Service == "gerrit" && profile.Provider != "keyring" {
							t.Fatal("keyring profile was not listed")
						}
					}
					if defaults != 5 {
						t.Fatal("service defaults not represented", defaults)
					}
				}
			} else {
				for _, service := range []string{"gerrit", "phabricator", "wiki", "cloudvps", "gitlab", "enterprise"} {
					if !strings.Contains(out.String(), service) {
						t.Fatal("missing service", service)
					}
				}
			}
		})
	}
}

func TestConfigProfilesEmptyAndInvalidService(t *testing.T) {
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_PROJECT_DIR", t.TempDir())
	if err := mwconfig.PutDiskConfig(koanf.New(".")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { mwconfig.ReApplyKoanfConf(koanf.New(".")) })
	cmd := Cmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"profiles", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != "[]" {
		t.Fatal("empty JSON is not an array", out.String())
	}
	cmd = Cmd()
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"profiles", "--service", "unknown"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("unknown service accepted")
	}
}
