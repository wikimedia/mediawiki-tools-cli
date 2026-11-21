package gerrit

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestGerritCheckout(t *testing.T) {
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	remote := t.TempDir()
	worktree := t.TempDir()
	git := func(dir string, args ...string) string {
		t.Helper()
		cmd := osexec.CommandContext(t.Context(), "git", args...)
		cmd.Dir = dir
		data, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, data)
		}
		return strings.TrimSpace(string(data))
	}
	git(remote, "init", "--quiet", "--initial-branch=main")
	git(remote, "config", "user.name", "Fixture")
	git(remote, "config", "user.email", "fixture@example.org")
	git(remote, "commit", "--quiet", "--allow-empty", "-m", "base")
	git(worktree, "clone", "--quiet", remote, ".")
	git(remote, "commit", "--quiet", "--allow-empty", "-m", "patchset four")
	older := git(remote, "rev-parse", "HEAD")
	git(remote, "update-ref", "refs/changes/81/1370181/4", older)
	git(remote, "commit", "--quiet", "--allow-empty", "-m", "patchset five")
	latest := git(remote, "rev-parse", "HEAD")
	git(remote, "update-ref", "refs/changes/81/1370181/5", latest)
	const canonical = "https://gerrit.wikimedia.org/r/mediawiki/extensions/WSOAuth"
	git(worktree, "remote", "set-url", "origin", "git@github.com:wikimedia/mediawiki-extensions-WSOAuth.git")
	git(worktree, "config", "url."+remote+".insteadOf", "https::"+canonical)
	t.Chdir(worktree)

	oldTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = oldTransport })
	http.DefaultTransport = transportFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "gerrit.wikimedia.org" || req.URL.Path != "/r/changes/1370181" || req.Header.Get("Authorization") != "" {
			t.Fatalf("unexpected Gerrit request: %s", req.URL)
		}
		revisions := fmt.Sprintf(`"%s":{"_number":5,"ref":"refs/changes/81/1370181/5"}`, latest)
		switch field := req.URL.Query().Get("o"); field {
		case "ALL_REVISIONS":
			revisions += fmt.Sprintf(`,"%s":{"_number":4,"ref":"refs/changes/81/1370181/4"}`, older)
		case "CURRENT_REVISION":
		default:
			t.Fatalf("unexpected revision selector: %q", field)
		}
		body := ")]}'\n" + fmt.Sprintf(`{"project":"mediawiki/extensions/WSOAuth","current_revision":"%s","revisions":{%s}}`, latest, revisions)
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
	})

	runTransfer := func(command string, args ...string) (string, error) {
		t.Helper()
		cmd := &cobra.Command{Use: "wm"}
		cmd.AddGroup(&cobra.Group{ID: "service", Title: "Service commands"})
		cmd.AddCommand(NewGerritCmd())
		var output bytes.Buffer
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		cmd.SetArgs(append([]string{"gerrit", command}, args...))
		err := cmd.ExecuteContext(t.Context())
		return output.String(), err
	}
	run := func(args ...string) (string, error) {
		return runTransfer("checkout", args...)
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"1370181"}, latest},
		{[]string{"1370181", "--patchset", "4"}, older},
		{[]string{"https://gerrit.wikimedia.org/r/c/mediawiki/extensions/WSOAuth/+/1370181/4"}, older},
	} {
		if out, err := run(test.args...); err != nil {
			t.Fatalf("checkout %v: %v\n%s", test.args, err, out)
		}
		if head := git(worktree, "rev-parse", "HEAD"); head != test.want {
			t.Fatalf("checkout %v landed on %s, want %s", test.args, head, test.want)
		}
		if branch := git(worktree, "rev-parse", "--abbrev-ref", "HEAD"); branch != "HEAD" {
			t.Fatalf("checkout left HEAD on branch %s", branch)
		}
	}
	if _, err := run("1370181", "--patchset", "3"); err == nil || !strings.Contains(err.Error(), "patchset 3 is unavailable") {
		t.Fatalf("expected missing patchset error, got %v", err)
	}
	// A clone configured for HTTPS stores the explicit Git transport as its remote.
	git(worktree, "remote", "set-url", "origin", "https::"+canonical)
	if out, err := run("1370181", "--patchset", "4"); err != nil {
		t.Fatalf("checkout with an explicit HTTPS remote: %v\n%s", err, out)
	}
	if head := git(worktree, "rev-parse", "HEAD"); head != older {
		t.Fatalf("checkout landed on %s, want %s", head, older)
	}
	git(worktree, "remote", "set-url", "origin", "https://gerrit.wikimedia.org/r/mediawiki/core")
	if _, err := run("1370181"); err == nil || !strings.Contains(err.Error(), "wm dev mediawiki apply-patches --change 1370181 --clone-missing") {
		t.Fatalf("expected wrong repository error, got %v", err)
	}
	if head := git(worktree, "rev-parse", "HEAD"); head != older {
		t.Fatalf("wrong repository checkout changed HEAD to %s", head)
	}
	if err := os.WriteFile(filepath.Join(worktree, "local.txt"), []byte("local work"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := run("1370181"); err == nil || !strings.Contains(err.Error(), "uncommitted changes") {
		t.Fatalf("expected dirty worktree error, got %v", err)
	}

	// Fetch works with a dirty worktree and updates only FETCH_HEAD and objects.
	git(worktree, "switch", "--quiet", "main")
	git(worktree, "add", "local.txt")
	if err := os.WriteFile(filepath.Join(worktree, "local.txt"), []byte("unstaged edit"), 0o600); err != nil {
		t.Fatal(err)
	}
	base := git(worktree, "rev-parse", "HEAD")
	status := git(worktree, "status", "--porcelain")
	refs := git(worktree, "for-each-ref")
	git(worktree, "remote", "set-url", "origin", "https::"+canonical)
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"1370181"}, latest},
		{[]string{"https://gerrit.wikimedia.org/r/c/mediawiki/extensions/WSOAuth/+/1370181/4"}, older},
	} {
		out, err := runTransfer("fetch", test.args...)
		if err != nil || !strings.Contains(out, "into FETCH_HEAD") {
			t.Fatalf("fetch %v: %v\n%s", test.args, err, out)
		}
		if fetched := git(worktree, "rev-parse", "FETCH_HEAD"); fetched != test.want {
			t.Fatalf("fetch %v wrote %s to FETCH_HEAD, want %s", test.args, fetched, test.want)
		}
		if git(worktree, "rev-parse", "HEAD") != base || git(worktree, "branch", "--show-current") != "main" ||
			git(worktree, "status", "--porcelain") != status || git(worktree, "for-each-ref") != refs {
			t.Fatal("fetch changed the checked-out branch, worktree, index or named refs")
		}
		data, err := os.ReadFile(filepath.Join(worktree, "local.txt"))
		if err != nil || string(data) != "unstaged edit" || git(worktree, "show", ":local.txt") != "local work" {
			t.Fatalf("fetch changed staged or unstaged file contents: %v", err)
		}
	}
	git(worktree, "remote", "set-url", "origin", "https://gerrit.wikimedia.org/r/mediawiki/core")
	if _, err := runTransfer("fetch", "1370181"); err == nil || !strings.Contains(err.Error(), "no matching remote") {
		t.Fatalf("expected wrong repository fetch error, got %v", err)
	}
	if got := git(worktree, "rev-parse", "FETCH_HEAD"); got != older {
		t.Fatalf("rejected fetch changed FETCH_HEAD to %s", got)
	}
}
