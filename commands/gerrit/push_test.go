package gerrit

import (
	"bytes"
	"io"
	"net/http"
	"os"
	osexec "os/exec"
	"strings"
	"testing"
)

func TestGerritPushPreviewWithDetachedHEADAndRemoteTipMissingLocally(t *testing.T) {
	setupAuthTest(t)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	origin, worktree := t.TempDir(), t.TempDir()
	git := func(dir string, args ...string) string {
		t.Helper()
		command := osexec.CommandContext(t.Context(), "git", args...)
		command.Dir = dir
		if data, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, data)
			return ""
		} else {
			return strings.TrimSpace(string(data))
		}
	}
	identity := func(dir string) {
		git(dir, "config", "user.name", "Reviewer")
		git(dir, "config", "user.email", "reviewer@example.test")
	}
	git(origin, "init", "--quiet", "--initial-branch=main")
	identity(origin)
	git(origin, "commit", "--quiet", "--allow-empty", "-m", "base")
	git(worktree, "clone", "--quiet", origin, ".")
	identity(worktree)

	const project = "test/gerrit-ping"
	fetch := gerritPushURL(project, "https", "")
	push := gerritPushURL(project, "ssh", "fixture")
	git(worktree, "remote", "set-url", "--push", "origin", push)
	git(worktree, "config", "url."+origin+".insteadOf", fetch)
	const id = "I0123456789012345678901234567890123456789"
	git(worktree, "commit", "--quiet", "--allow-empty", "-m", "First proposed change", "-m", "Change-Id: "+id)
	git(worktree, "commit", "--quiet", "--allow-empty", "-m", "Second proposed change")
	head := git(worktree, "rev-parse", "HEAD")
	git(worktree, "checkout", "--quiet", "--detach")
	git(origin, "commit", "--quiet", "--allow-empty", "-m", "Gerrit branch advanced")
	tip := git(origin, "rev-parse", "HEAD")
	t.Chdir(worktree)

	http.DefaultTransport = transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet || r.URL.Path != "/r/changes/" || r.Header.Get("Authorization") != "" {
			t.Errorf("unexpected Gerrit request: %s %s", r.Method, r.URL)
		}
		var response string
		switch query := r.URL.Query().Get("q"); {
		case strings.Contains(query, "commit:"+head):
			response = `[{"project":"test/gerrit-ping","branch":"main","_number":42}]`
		case strings.Contains(query, "change:"+id) && strings.Contains(query, `branch:"main"`):
			response = `[{"project":"test/gerrit-ping","branch":"main","_number":42,"status":"NEW"}]`
		default:
			t.Errorf("unexpected Gerrit query %q", query)
			response = "[]"
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(")]}'\n" + response)), Request: r}, nil
	})
	var out bytes.Buffer
	cmd := NewGerritCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"push", "--dry-run"})
	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	for _, want := range []string{
		push, "Target: refs/for/main", "First proposed change", id,
		"Would update review 42: https://gerrit.wikimedia.org/r/42", "Second proposed change", "Missing Change-Id",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in:\n%s", want, out.String())
		}
	}
	if err := osexec.Command("git", "cat-file", "-e", tip).Run(); err == nil {
		t.Error("preview left remote objects in the checkout")
	}
	if _, err := os.Stat(".git/FETCH_HEAD"); !os.IsNotExist(err) {
		t.Errorf("preview should leave FETCH_HEAD absent, got %v", err)
	}
	if git(origin, "rev-parse", "HEAD") != tip || git(worktree, "rev-parse", "HEAD") != head {
		t.Error("preview changed the local or remote branch")
	}
}
