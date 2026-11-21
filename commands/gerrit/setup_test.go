package gerrit

import (
	"bytes"
	"io"
	"net/http"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	gerritapi "gitlab.wikimedia.org/repos/releng/cli/internal/gerrit"
)

func TestGerritSetupUsesEffectivePushURLAndHookPath(t *testing.T) {
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Chdir(t.TempDir())
	git := func(args ...string) {
		t.Helper()
		if output, err := osexec.CommandContext(t.Context(), "git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	git("init", "--quiet")
	git("config", "user.name", "Gerrit User")
	git("config", "user.email", "developer@example.test")
	git("remote", "add", "origin", "https://gerrit.wikimedia.org/r/test/gerrit-ping")
	git("config", "url.ssh://developer@gerrit.wikimedia.org:29418/.pushInsteadOf", "https://gerrit.wikimedia.org/r/")
	git("config", "core.hooksPath", "custom-hooks")
	if err := os.Mkdir("custom-hooks", 0o755); err != nil {
		t.Fatal(err)
	}
	hook := filepath.Join("custom-hooks", "commit-msg")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetContext(t.Context())
	targets := checkGerritCheckout(cmd)
	if len(targets) != 1 || targets[0] != (gerritSSHTarget{user: "developer", port: "29418"}) {
		t.Fatalf("unexpected SSH targets: %+v\n%s", targets, output.String())
	}
	for _, expected := range []string{
		"(fetch): https://gerrit.wikimedia.org/r/test/gerrit-ping",
		"(push): ssh://developer@gerrit.wikimedia.org:29418/test/gerrit-ping",
		"Git author email: developer@example.test",
		"commit-msg hook: found at",
	} {
		if !strings.Contains(output.String(), expected) {
			t.Errorf("missing %q in output:\n%s", expected, output.String())
		}
	}
}

func TestGerritCloneAndConfigurePush(t *testing.T) {
	setupAuthTest(t)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	source, parent := t.TempDir(), t.TempDir()
	git := func(directory string, args ...string) string {
		t.Helper()
		if result, err := gitOutput(t.Context(), directory, args...); err != nil {
			t.Fatalf("git %v: %v", args, err)
			return ""
		} else {
			return result
		}
	}
	git(source, "init", "--quiet", "--initial-branch=main")

	const project = "test/gerrit-ping"
	fetch := gerritPushURL(project, "https", "")
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "url."+source+".insteadOf")
	t.Setenv("GIT_CONFIG_VALUE_0", fetch)

	hookRequests := 0
	http.DefaultTransport = transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet || r.URL.String() != gerritapi.CommitHookURL {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		hookRequests++
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("#!/bin/sh\nexit 0\n")), Request: r}, nil
	})
	run := func(args ...string) string {
		t.Helper()
		var output bytes.Buffer
		cmd := NewGerritCmd()
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		cmd.SetArgs(args)
		if err := cmd.ExecuteContext(t.Context()); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, output.String())
		}
		return output.String()
	}
	t.Chdir(parent)
	run("clone", project, "clone", "--username", "fixture")
	t.Setenv("GIT_CONFIG_COUNT", "0")

	clone := filepath.Join(parent, "clone")
	if got := git(clone, "remote", "get-url", "origin"); got != fetch {
		t.Fatalf("clone fetch URL = %q, want %q", got, fetch)
	}
	expectedSSH := gerritPushURL(project, "ssh", "fixture")
	if got := git(clone, "remote", "get-url", "--push", "origin"); got != expectedSSH {
		t.Fatalf("clone push URL = %q, want %q", got, expectedSSH)
	}
	t.Chdir(clone)
	if out := run("setup", "--install-hook"); hookRequests != 1 || !strings.Contains(out, "commit-msg hook: found") {
		t.Fatalf("expected existing hook, made %d download(s) and got %s", hookRequests, out)
	}
	run("setup", "--push-protocol", "https")
	if got := git(clone, "remote", "get-url", "--push", "origin"); got != fetch {
		t.Fatalf("configured push URL = %q, want %q", got, fetch)
	}
	if got := git(clone, "remote", "get-url", "origin"); got != fetch {
		t.Fatalf("configuring push changed fetch URL to %q", got)
	}
}
