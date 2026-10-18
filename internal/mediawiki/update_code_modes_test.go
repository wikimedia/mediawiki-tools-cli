package mediawiki

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeUpdateFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func dirtyUpdateCheckout(t *testing.T, dir string, conflict bool) {
	t.Helper()
	name := "mixed changes.txt"
	if conflict {
		name = "file"
	}
	writeUpdateFile(t, dir, name, "staged\n")
	updateTestGit(t, dir, "add", name)
	writeUpdateFile(t, dir, name, "unstaged\n")
	writeUpdateFile(t, dir, "untracked file.txt", "untracked\x00bytes\n")
}

func updateCheckoutState(t *testing.T, dir string) string {
	t.Helper()
	return updateTestGit(t, dir, "rev-parse", "HEAD") + "\n" +
		updateTestGit(t, dir, "symbolic-ref", "--short", "HEAD") + "\n" +
		updateTestGit(t, dir, "status", "--porcelain", "--untracked-files=all") + "\n" +
		updateTestGit(t, dir, "diff", "--binary") + "\n" +
		updateTestGit(t, dir, "diff", "--cached", "--binary")
}

func assertUpdateDirtyFiles(t *testing.T, dir string, conflict bool) {
	t.Helper()
	name := "mixed changes.txt"
	if conflict {
		name = "file"
	}
	for name, want := range map[string]string{name: "unstaged\n", "untracked file.txt": "untracked\x00bytes\n"} {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(got) != want {
			t.Fatalf("%s: got %q, err=%v", name, got, err)
		}
	}
}

func runUpdateMode(t *testing.T, dir, mode string, dryRun bool) ([]SourceUpdate, error) {
	t.Helper()
	return UpdateSourceRepositoriesWithOptions(context.Background(), []SourceRepository{{Name: "core", Directory: dir}}, SourceUpdateOptions{Mode: mode, DryRun: dryRun})
}

func TestUpdateModesDryRunDoesNotFetchOrModify(t *testing.T) {
	for _, mode := range []string{"ff-only", "stash", "mop"} {
		t.Run(mode, func(t *testing.T) {
			_, _, dir := updateTestFixture(t)
			dirtyUpdateCheckout(t, dir, false)
			updateTestGit(t, dir, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing.git"))
			before := updateCheckoutState(t, dir)
			refs := updateTestGit(t, dir, "for-each-ref")
			if _, err := runUpdateMode(t, dir, mode, true); err != nil {
				t.Fatal(err)
			}
			if got := updateCheckoutState(t, dir); got != before {
				t.Fatalf("dry-run changed checkout\nbefore=%s\nafter=%s", before, got)
			}
			if got := updateTestGit(t, dir, "for-each-ref"); got != refs {
				t.Fatal("dry-run changed refs")
			}
			assertUpdateDirtyFiles(t, dir, false)
		})
	}
}

func TestStashUpdatePreservesIndexWorktreeAndExistingStash(t *testing.T) {
	_, seed, dir := updateTestFixture(t)
	writeUpdateFile(t, dir, "previous-note", "old stash")
	updateTestGit(t, dir, "stash", "push", "-u", "-m", "existing stash")
	oldStash := updateTestGit(t, dir, "rev-parse", "refs/stash")
	dirtyUpdateCheckout(t, dir, false)
	staged := updateTestGit(t, dir, "diff", "--cached", "--binary")
	unstaged := updateTestGit(t, dir, "diff", "--binary")
	results, err := runUpdateMode(t, dir, "stash", false)
	if err != nil || results[0].Status != "updated-stash" {
		t.Fatalf("%+v: %v", results, err)
	}
	if updateTestGit(t, dir, "rev-parse", "HEAD") != updateTestGit(t, seed, "rev-parse", "HEAD") {
		t.Fatal("not updated")
	}
	if updateTestGit(t, dir, "diff", "--cached", "--binary") != staged || updateTestGit(t, dir, "diff", "--binary") != unstaged {
		t.Fatal("staging/worktree changed")
	}
	if !strings.Contains(updateTestGit(t, dir, "stash", "list", "--format=%H"), oldStash) {
		t.Fatal("existing stash lost")
	}
	if !strings.Contains(updateTestGit(t, dir, "for-each-ref", "--format=%(refname)", "refs/mwcli/update-code"), "/stash") {
		t.Fatal("no stash recovery ref")
	}
	assertUpdateDirtyFiles(t, dir, false)
}

func TestStashUpdateConflictRestoresExactCheckout(t *testing.T) {
	_, _, dir := updateTestFixture(t)
	dirtyUpdateCheckout(t, dir, true)
	before := updateCheckoutState(t, dir)
	results, err := runUpdateMode(t, dir, "stash", false)
	if err == nil || results[0].Status != "restored" {
		t.Fatalf("%+v: %v", results, err)
	}
	if got := updateCheckoutState(t, dir); got != before {
		t.Fatalf("checkout not restored\nbefore=%s\nafter=%s", before, got)
	}
	assertUpdateDirtyFiles(t, dir, true)
	if strings.Count(updateTestGit(t, dir, "worktree", "list", "--porcelain"), "worktree ") != 1 {
		t.Fatal("temporary worktree left registered")
	}
}

func TestMopSavesChangesAndCommitsOnDefaultBranch(t *testing.T) {
	for _, name := range []string{"main", "master"} {
		t.Run(name, func(t *testing.T) {
			remote, seed, dir := updateTestFixture(t)
			if name == "master" {
				updateTestGit(t, seed, "branch", "-m", "master")
				updateTestGit(t, seed, "push", "-u", "origin", "master")
				updateTestGit(t, remote, "symbolic-ref", "HEAD", "refs/heads/master")
			}
			writeUpdateFile(t, dir, "local-commit", "preserve me")
			updateTestGit(t, dir, "add", "local-commit")
			updateTestGit(t, dir, "commit", "-m", "local main commit")
			oldDefault := updateTestGit(t, dir, "rev-parse", "HEAD")
			updateTestGit(t, dir, "checkout", "-b", "feature")
			writeUpdateFile(t, dir, "feature-commit", "preserve feature")
			updateTestGit(t, dir, "add", "feature-commit")
			updateTestGit(t, dir, "commit", "-m", "local feature commit")
			before := updateTestGit(t, dir, "rev-parse", "HEAD")
			dirtyUpdateCheckout(t, dir, false)
			results, err := runUpdateMode(t, dir, "mop", false)
			if err != nil || results[0].Status != "updated-mop" {
				t.Fatalf("%+v: %v", results, err)
			}
			if updateTestGit(t, dir, "symbolic-ref", "--short", "HEAD") != name {
				t.Fatal("wrong default branch")
			}
			if updateTestGit(t, dir, "rev-parse", "HEAD") != updateTestGit(t, seed, "rev-parse", "HEAD") {
				t.Fatal("not reset to remote")
			}
			if updateTestGit(t, dir, "status", "--porcelain") != "" {
				t.Fatal("not clean")
			}
			refs := updateTestGit(t, dir, "for-each-ref", "--format=%(objectname) %(refname)", "refs/mwcli/update-code")
			if !strings.Contains(refs, before) || !strings.Contains(refs, "/stash") {
				t.Fatal("original commit/stash not saved")
			}
			if name == "main" && !strings.Contains(refs, oldDefault) {
				t.Fatal("old default tip not saved")
			}
			output := updateTestGit(t, dir, "show", "refs/stash^3:untracked file.txt")
			if output != "untracked\x00bytes" {
				t.Fatalf("untracked file not saved: %q", output)
			}
		})
	}
}

func TestStashDivergenceLeavesDirtyCheckoutUntouched(t *testing.T) {
	_, _, dir := updateTestFixture(t)
	writeUpdateFile(t, dir, "local", "commit")
	updateTestGit(t, dir, "add", "local")
	updateTestGit(t, dir, "commit", "-m", "diverge")
	dirtyUpdateCheckout(t, dir, false)
	before := updateCheckoutState(t, dir)
	results, err := runUpdateMode(t, dir, "stash", false)
	if err == nil || results[0].Status != "failed" {
		t.Fatalf("%+v: %v", results, err)
	}
	if got := updateCheckoutState(t, dir); got != before {
		t.Fatal("diverged checkout changed")
	}
	if updateTestGit(t, dir, "stash", "list") != "" {
		t.Fatal("unexpected stash")
	}
}

func TestStashModesRefuseIgnoredFileCollision(t *testing.T) {
	for _, mode := range []string{"stash", "mop"} {
		t.Run(mode, func(t *testing.T) {
			_, seed, dir := updateTestFixture(t)
			writeUpdateFile(t, seed, "ignored-file", "remote")
			updateTestGit(t, seed, "add", "ignored-file")
			updateTestGit(t, seed, "commit", "-m", "new tracked path")
			updateTestGit(t, seed, "push")
			writeUpdateFile(t, dir, ".git/info/exclude", "ignored-file\n")
			writeUpdateFile(t, dir, "ignored-file", "local ignored data")
			before := updateCheckoutState(t, dir)
			_, err := runUpdateMode(t, dir, mode, false)
			if err == nil || !strings.Contains(err.Error(), "ignored path") {
				t.Fatalf("expected ignored-file refusal, got %v", err)
			}
			if updateCheckoutState(t, dir) != before {
				t.Fatal("checkout changed")
			}
			got, err := os.ReadFile(filepath.Join(dir, "ignored-file"))
			if err != nil || string(got) != "local ignored data" {
				t.Fatal("ignored data lost")
			}
		})
	}
}

func TestRollbackAfterFailedApplyRestoresIndexAndUntrackedFiles(t *testing.T) {
	_, _, dir := updateTestFixture(t)
	dirtyUpdateCheckout(t, dir, true)
	before := updateCheckoutState(t, dir)
	snapshot := sourceSnapshot{head: updateTestGit(t, dir, "rev-parse", "HEAD"), branch: "main", backup: "refs/mwcli/test"}
	updateTestGit(t, dir, "stash", "push", "-u")
	snapshot.stash = updateTestGit(t, dir, "rev-parse", "refs/stash")
	updateTestGit(t, dir, "fetch", "origin")
	updateTestGit(t, dir, "merge", "--ff-only", "origin/main")
	// Simulate a failure in the real apply, after the checkout has advanced.
	_, _ = sourceGit(context.Background(), dir, "stash", "apply", snapshot.stash)
	result, err := rollbackSourceUpdate(context.Background(), dir, snapshot, SourceUpdate{Name: "core"}, fmt.Errorf("simulated apply failure"), true)
	if err == nil || result.Status != "restored" {
		t.Fatalf("%+v: %v", result, err)
	}
	if got := updateCheckoutState(t, dir); got != before {
		t.Fatalf("rollback mismatch\nbefore=%s\nafter=%s", before, got)
	}
	assertUpdateDirtyFiles(t, dir, true)
}

func TestMopCannotTakeOverAnotherWorktree(t *testing.T) {
	_, _, dir := updateTestFixture(t)
	updateTestGit(t, dir, "checkout", "-b", "feature")
	other := filepath.Join(t.TempDir(), "other")
	updateTestGit(t, dir, "worktree", "add", other, "main")
	otherBefore := updateCheckoutState(t, other)
	dirtyUpdateCheckout(t, dir, false)
	before := updateCheckoutState(t, dir)
	results, err := runUpdateMode(t, dir, "mop", false)
	if err == nil || results[0].Status != "restored" {
		t.Fatalf("%+v: %v", results, err)
	}
	if updateCheckoutState(t, dir) != before || updateCheckoutState(t, other) != otherBefore {
		t.Fatal("worktree state changed")
	}
	assertUpdateDirtyFiles(t, dir, false)
}
