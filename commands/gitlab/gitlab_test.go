package gitlab

import (
	"io"
	"net/http"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/gitlab-org/cli/mwcli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

func testFactory(t *testing.T) mwcli.Factory {
	t.Helper()
	t.Chdir(t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GITLAB_HOST", "gitlab.wikimedia.org")
	t.Setenv("DEBUG", "")
	for _, key := range []string{"GITLAB_TOKEN", "GITLAB_ACCESS_TOKEN", "OAUTH_TOKEN", "CI_JOB_TOKEN", "GITLAB_REPO"} {
		t.Setenv(key, "")
	}
	configDir := t.TempDir()
	t.Setenv("GLAB_CONFIG_DIR", configDir)
	if err := os.WriteFile(filepath.Join(configDir, "config.yml"), []byte("hosts:\n  gitlab.wikimedia.org:\n    token: canonical-test-token\n  gitlab-ssh.wikimedia.org:\n    token: wrong-test-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, "init", "--quiet")
	factory, err := mwcli.NewFactory(glabVersion(), cli.HTTPUserAgent)
	if err != nil {
		t.Fatal(err)
	}
	return factory
}

func runGit(t *testing.T, args ...string) string {
	t.Helper()
	output, err := osexec.Command("git", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
	return string(output)
}

func TestWikimediaRemoteHosts(t *testing.T) {
	for _, remoteURL := range []string{
		"git@gitlab-ssh.wikimedia.org:repos/releng/cli.git",
		"ssh://git@gitlab-ssh.wikimedia.org:22/repos/releng/cli.git",
		"git@gitlab.wikimedia.org:repos/releng/cli.git",
		"https://gitlab.wikimedia.org/repos/releng/cli.git",
	} {
		t.Run(remoteURL, func(t *testing.T) {
			factory := testFactory(t)
			runGit(t, "remote", "add", "origin", remoteURL)
			runGit(t, "remote", "add", "github", "git@github.com:addshore/cli.git")
			before := runGit(t, "remote", "-v")
			remotes, err := factory.Remotes()
			if err != nil {
				t.Fatal(err)
			}
			if len(remotes) != 1 || remotes[0].Name != "origin" || remotes[0].RepoHost() != "gitlab.wikimedia.org" {
				t.Fatalf("wrong remote selection: %v", remotes)
			}
			expectedURL, err := mwcli.ParseURL(remoteURL)
			if err != nil {
				t.Fatal(err)
			}
			if remotes[0].FetchURL.String() != expectedURL.String() || remotes[0].PushURL.String() != expectedURL.String() {
				t.Fatal("SSH transport URL was changed")
			}
			repo, err := factory.BaseRepo()
			if err != nil || repo.RepoHost() != "gitlab.wikimedia.org" || repo.FullName() != "repos/releng/cli" {
				t.Fatalf("wrong base repo: %v, %v", repo, err)
			}
			client, err := factory.GitLabClient()
			if err != nil {
				t.Fatal(err)
			}
			client.HTTPClient().Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if !strings.HasPrefix(request.UserAgent(), cli.HTTPUserAgent+" glab/") {
					t.Errorf("GitLab User-Agent = %q", request.UserAgent())
				}
				if request.URL.Host != "gitlab.wikimedia.org" || request.Header.Get("PRIVATE-TOKEN") != "canonical-test-token" {
					t.Error("API did not use the canonical host and its credentials")
				}
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("{}")), Request: request}, nil
			})
			if client.BaseURL().Host != "gitlab.wikimedia.org" {
				t.Fatalf("wrong API host: %s", client.BaseURL())
			}
			request, err := client.NewRequest("GET", "version", nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := client.Do(request, nil); err != nil {
				t.Fatal(err)
			}
			if after := runGit(t, "remote", "-v"); after != before {
				t.Fatal("Git remotes were modified")
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestWikimediaMixedRemotes(t *testing.T) {
	factory := testFactory(t)
	runGit(t, "remote", "add", "origin", "git@gitlab-ssh.wikimedia.org:addshore/cli.git")
	runGit(t, "remote", "add", "upstream", "https://gitlab.wikimedia.org/repos/releng/cli.git")
	remotes, err := factory.Remotes()
	if err != nil {
		t.Fatal(err)
	}
	if len(remotes) != 2 || remotes[0].Name != "upstream" || remotes[1].Name != "origin" {
		t.Fatalf("expected both Wikimedia remotes in glab's usual order: %v", remotes)
	}
}

func TestWikimediaRepoOverride(t *testing.T) {
	factory := testFactory(t)
	for _, override := range []string{
		"git@gitlab-ssh.wikimedia.org:repos/releng/cli.git",
		"https://gitlab.wikimedia.org/repos/releng/cli",
		"gitlab-ssh.wikimedia.org/repos/releng/cli",
	} {
		if err := factory.RepoOverride(override); err != nil {
			t.Fatal(err)
		}
		repo, err := factory.BaseRepo()
		if err != nil || repo.RepoHost() != "gitlab.wikimedia.org" || repo.FullName() != "repos/releng/cli" {
			t.Fatalf("override %s: %v, %v", override, repo, err)
		}
	}
}

func TestUnrelatedRemoteIsRejected(t *testing.T) {
	factory := testFactory(t)
	runGit(t, "remote", "add", "origin", "git@gitlab-ssh.wikimedia.org.example:repos/releng/cli.git")
	if _, err := factory.Remotes(); err == nil {
		t.Fatal("unrelated host was accepted as Wikimedia GitLab")
	}
}

func TestWikimediaDefaultHostAndAPIClient(t *testing.T) {
	testFactory(t)
	t.Setenv("GITLAB_HOST", "")
	factory, err := mwcli.NewFactory(glabVersion(), cli.HTTPUserAgent)
	if err != nil {
		t.Fatal(err)
	}
	if factory.DefaultHostname() != "gitlab.wikimedia.org" {
		t.Fatalf("default hostname = %q", factory.DefaultHostname())
	}
	client, err := factory.ApiClient("gitlab-ssh.wikimedia.org")
	if err != nil {
		t.Fatal(err)
	}
	if client.Lab().BaseURL().Host != "gitlab.wikimedia.org" {
		t.Fatalf("API host = %q", client.Lab().BaseURL().Host)
	}
	if !strings.HasPrefix(client.Lab().UserAgent, cli.HTTPUserAgent+" glab/") {
		t.Fatalf("API User-Agent = %q", client.Lab().UserAgent)
	}
	root := Cmd()
	for _, command := range root.Commands() {
		if command.Name() == "update" || command.Name() == "check-update" {
			t.Fatal("embedded glab exposes its standalone updater")
		}
	}
}

func TestGitLabConfigurationError(t *testing.T) {
	testFactory(t)
	if err := os.WriteFile(filepath.Join(os.Getenv("GLAB_CONFIG_DIR"), "config.yml"), []byte("hosts: ["), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := Cmd()
	cmd.SetArgs([]string{"api", "version"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "read GitLab configuration") {
		t.Fatalf("configuration error = %v", err)
	}
}
