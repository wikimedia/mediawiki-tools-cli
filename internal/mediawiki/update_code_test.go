package mediawiki

import (
	"context"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func updateTestGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := append([]string{"-C", dir, "-c", "user.name=addshore", "-c", "user.email=addshore@users.noreply.github.com"}, args...)
	output, err := osexec.Command("git", command...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func updateTestFixture(t *testing.T) (remote, seed, checkout string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(dir, "no-config"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	remote = filepath.Join(dir, "remote.git")
	seed = filepath.Join(dir, "seed")
	checkout = filepath.Join(dir, "checkout")
	updateTestGit(t, dir, "-c", "init.defaultBranch=main", "init", "--bare", remote)
	updateTestGit(t, dir, "clone", remote, seed)
	if err := os.WriteFile(filepath.Join(seed, "file"), []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	updateTestGit(t, seed, "add", "file")
	updateTestGit(t, seed, "commit", "-m", "Initial fixture")
	updateTestGit(t, seed, "push", "-u", "origin", "main")
	updateTestGit(t, dir, "clone", remote, checkout)
	if err := os.WriteFile(filepath.Join(seed, "file"), []byte("second"), 0600); err != nil {
		t.Fatal(err)
	}
	updateTestGit(t, seed, "commit", "-am", "Remote fixture update")
	updateTestGit(t, seed, "push")
	return
}

func TestUpdateCodeDryRunAndFastForward(t *testing.T) {
	_, seed, checkout := updateTestFixture(t)
	before := updateTestGit(t, checkout, "rev-parse", "HEAD")
	repos := []SourceRepository{{Name: "core", Directory: checkout}}
	results, err := UpdateSourceRepositories(context.Background(), repos, true)
	if err != nil || results[0].Status != "would-pull-ff-only" {
		t.Fatalf("%+v %v", results, err)
	}
	if updateTestGit(t, checkout, "rev-parse", "HEAD") != before {
		t.Fatal("dry run changed HEAD")
	}
	results, err = UpdateSourceRepositories(context.Background(), repos, false)
	if err != nil || results[0].Status != "updated" {
		t.Fatalf("%+v %v", results, err)
	}
	if updateTestGit(t, checkout, "rev-parse", "HEAD") != updateTestGit(t, seed, "rev-parse", "HEAD") {
		t.Fatal("not fast-forwarded")
	}
}

func TestUpdateCodeSkipsAndDivergence(t *testing.T) {
	for _, mode := range []string{"dirty", "untracked", "detached", "no-upstream", "diverged"} {
		t.Run(mode, func(t *testing.T) {
			_, _, checkout := updateTestFixture(t)
			want := "skipped-" + mode
			switch mode {
			case "dirty":
				if err := os.WriteFile(filepath.Join(checkout, "file"), []byte("local"), 0600); err != nil {
					t.Fatal(err)
				}
			case "untracked":
				if err := os.WriteFile(filepath.Join(checkout, "untracked"), []byte("local"), 0600); err != nil {
					t.Fatal(err)
				}
				want = "skipped-dirty"
			case "detached":
				updateTestGit(t, checkout, "checkout", "--detach")
			case "no-upstream":
				updateTestGit(t, checkout, "branch", "--unset-upstream")
			case "diverged":
				if err := os.WriteFile(filepath.Join(checkout, "local"), []byte("local"), 0600); err != nil {
					t.Fatal(err)
				}
				updateTestGit(t, checkout, "add", "local")
				updateTestGit(t, checkout, "commit", "-m", "Local fixture commit")
				want = "failed"
			}
			before := updateTestGit(t, checkout, "rev-parse", "HEAD")
			results, err := UpdateSourceRepositories(context.Background(), []SourceRepository{{Name: mode, Directory: checkout}}, false)
			if results[0].Status != want || (err != nil) != (mode == "diverged") {
				t.Fatalf("%+v %v", results, err)
			}
			if updateTestGit(t, checkout, "rev-parse", "HEAD") != before {
				t.Fatal("changed protected checkout")
			}
		})
	}
}

func TestSourceRepositoryDiscoveryAndCancellation(t *testing.T) {
	_, _, checkout := updateTestFixture(t)
	for _, sub := range []string{"skins/Example", "extensions/Example"} {
		updateTestGit(t, checkout, "init", filepath.Join(checkout, sub))
	}
	if err := os.MkdirAll(filepath.Join(checkout, "extensions/PlainDirectory"), 0755); err != nil {
		t.Fatal(err)
	}
	repos, err := MediaWiki(checkout).SourceRepositories()
	if err != nil || len(repos) != 3 {
		t.Fatalf("%+v %v", repos, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := UpdateSourceRepositories(ctx, repos, false); err != context.Canceled {
		t.Fatal(err)
	}
}
