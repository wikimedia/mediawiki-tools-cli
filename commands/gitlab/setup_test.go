package gitlab

import (
	"io"
	"net/http"
	"os"
	osexec "os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type setupTransport func(*http.Request) (*http.Response, error)

func (f setupTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestGitlabSetupUsesEffectivePushTransport(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SSH fixture uses a POSIX shell")
	}
	isolateGitlabProfiles(t)
	savedGitlabProfile(t, "fixture", "tester", "fixture-token")
	old := http.DefaultTransport
	http.DefaultTransport = setupTransport(func(request *http.Request) (*http.Response, error) {
		if request.Method != "GET" || request.URL.Host != "gitlab.wikimedia.org" || request.URL.Path != "/api/v4/user" {
			t.Errorf("unexpected GitLab request: %s %s", request.Method, request.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"username":"tester"}`)), Request: request}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = old })
	t.Chdir(t.TempDir())
	for _, args := range [][]string{
		{"init", "--quiet"},
		{"remote", "add", "review", "https://gitlab.wikimedia.org/group/project.git"},
		{"config", "url.ssh://git@gitlab-ssh.wikimedia.org/.pushInsteadOf", "https://gitlab.wikimedia.org/"},
	} {
		if data, err := osexec.CommandContext(t.Context(), "git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, data)
		}
	}
	ssh := filepath.Join(t.TempDir(), "ssh")
	if err := os.WriteFile(ssh, []byte("#!/bin/sh\ncase \"$*\" in\n*gitlab-ssh.wikimedia.org*git-upload-pack*group/project.git*) exec git upload-pack . ;;\n*) exit 97 ;;\nesac\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_SSH_COMMAND", shellQuote(ssh))
	t.Setenv("GIT_SSH_VARIANT", "ssh")
	out, err := runProfileCommand("", "--profile", "fixture", "setup", "--check")
	if err != nil {
		t.Fatalf("setup failed: %v\n%s", err, out)
	}
	for _, expected := range []string{
		"authenticated as tester", "Git remote review (fetch): https://gitlab.wikimedia.org/group/project.git",
		"(push): ssh://git@gitlab-ssh.wikimedia.org/group/project.git", "Git SSH access: repository is readable",
	} {
		if !strings.Contains(out, expected) {
			t.Errorf("missing %q from %q", expected, out)
		}
	}
	if strings.Contains(out, "fixture-token") {
		t.Fatal("setup exposed the profile token")
	}
}
