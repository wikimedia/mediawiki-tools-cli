package mediawiki

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SourceUpdateOptions keeps destructive default-branch resets explicitly opt-in.
type SourceUpdateOptions struct {
	DryRun bool
	Mode   string
}

type sourceRecoveryError struct{ err error }

func (e *sourceRecoveryError) Error() string { return e.err.Error() }
func (e *sourceRecoveryError) Unwrap() error { return e.err }

type sourceSnapshot struct {
	head, branch, stash, backup string
	targetBranch, targetHead    string
	targetExisted               bool
}

func sourceOperationInProgress(ctx context.Context, dir string) error {
	for _, name := range []string{"MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "rebase-merge", "rebase-apply", "sequencer", "index.lock"} {
		path, err := sourceGit(ctx, dir, "rev-parse", "--git-path", name)
		if err != nil {
			return err
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(dir, path)
		}
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("Git operation in progress (%s); finish it before updating", name)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func sourceDefaultBranch(ctx context.Context, dir string, dryRun bool) (string, error) {
	if !dryRun {
		// Ask the remote itself; origin/HEAD may be missing or stale.
		output, err := sourceGit(ctx, dir, "ls-remote", "--symref", "origin", "HEAD")
		if err != nil {
			return "", err
		}
		for _, line := range strings.Split(output, "\n") {
			fields := strings.Fields(line)
			if len(fields) == 3 && fields[0] == "ref:" && fields[2] == "HEAD" && strings.HasPrefix(fields[1], "refs/heads/") {
				return strings.TrimPrefix(fields[1], "refs/heads/"), nil
			}
		}
		return "", fmt.Errorf("origin did not advertise its default branch")
	}
	if name, err := sourceGit(ctx, dir, "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil {
		return strings.TrimPrefix(name, "origin/"), nil
	}
	for _, name := range []string{"main", "master"} {
		if _, err := sourceGit(ctx, dir, "show-ref", "--verify", "refs/remotes/origin/"+name); err == nil {
			return name, nil
		}
	}
	return "", fmt.Errorf("default branch is unknown locally; dry-run will not contact origin")
}

func updateStashedSource(ctx context.Context, repo SourceRepository, options SourceUpdateOptions) (SourceUpdate, error) {
	result := SourceUpdate{Name: repo.Name, Status: "failed"}
	dir := repo.Directory
	if err := sourceOperationInProgress(ctx, dir); err != nil {
		return result, err
	}
	before, err := sourceGit(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		return result, err
	}
	branch, branchErr := sourceGit(ctx, dir, "symbolic-ref", "--quiet", "--short", "HEAD")
	snapshot := sourceSnapshot{head: before, branch: branch}
	result.Branch = branch
	var target string
	if options.Mode == "mop" {
		snapshot.targetBranch, err = sourceDefaultBranch(ctx, dir, options.DryRun)
		if err != nil {
			return result, err
		}
		result.Upstream = "origin/" + snapshot.targetBranch
	} else {
		if branchErr != nil {
			result.Status = "skipped-detached"
			return result, nil
		}
		result.Upstream, err = sourceGit(ctx, dir, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
		if err != nil {
			result.Status = "skipped-no-upstream"
			return result, nil
		}
	}
	if options.DryRun {
		result.Status = "would-" + options.Mode
		if options.Mode == "mop" {
			result.Detail = "Would save changes and recovery refs, then reset to " + result.Upstream + "; default branch is inferred from local refs"
		} else {
			result.Detail = "Would save changes, fast-forward, and reapply the saved index and worktree"
		}
		return result, nil
	}
	if options.Mode == "mop" {
		_, err = sourceGit(ctx, dir, "fetch", "--no-recurse-submodules", "origin")
	} else {
		remote, remoteErr := sourceGit(ctx, dir, "config", "--get", "branch."+branch+".remote")
		if remoteErr != nil {
			return result, remoteErr
		}
		_, err = sourceGit(ctx, dir, "fetch", "--no-recurse-submodules", remote)
	}
	if err != nil {
		return result, err
	}
	target, err = sourceGit(ctx, dir, "rev-parse", "--verify", result.Upstream+"^{commit}")
	if err != nil {
		return result, err
	}
	if options.Mode == "stash" {
		if _, err := sourceGit(ctx, dir, "merge-base", "--is-ancestor", before, target); err != nil {
			if _, aheadErr := sourceGit(ctx, dir, "merge-base", "--is-ancestor", target, before); aheadErr != nil {
				return result, fmt.Errorf("branch has diverged from %s; no changes were stashed or reset", result.Upstream)
			}
			// A branch already ahead of its upstream needs no update.
			target = before
		}
	}
	if err := sourceIgnoredCollisions(ctx, dir, target); err != nil {
		return result, err
	}
	snapshot.backup = fmt.Sprintf("refs/mwcli/update-code/%d", time.Now().UnixNano())
	if _, err := sourceGit(ctx, dir, "update-ref", snapshot.backup+"/original", before, ""); err != nil {
		return result, err
	}
	if options.Mode == "mop" {
		if old, err := sourceGit(ctx, dir, "rev-parse", "--verify", "refs/heads/"+snapshot.targetBranch); err == nil {
			snapshot.targetHead, snapshot.targetExisted = old, true
			if _, err := sourceGit(ctx, dir, "update-ref", snapshot.backup+"/default", old, ""); err != nil {
				return result, err
			}
		}
	}
	result.Detail = "Recovery commit: " + snapshot.backup + "/original"
	if snapshot.targetExisted {
		result.Detail += "; previous default branch: " + snapshot.backup + "/default"
	}
	dirty, err := sourceGit(ctx, dir, "status", "--porcelain", "--untracked-files=all", "--ignore-submodules=none")
	if err != nil {
		return result, err
	}
	if dirty != "" {
		oldStash, _ := sourceGit(ctx, dir, "rev-parse", "--verify", "refs/stash")
		_, stashErr := sourceGit(ctx, dir, "stash", "push", "--include-untracked", "--message", "mwcli update-code "+options.Mode+" "+snapshot.backup)
		newStash, _ := sourceGit(ctx, dir, "rev-parse", "--verify", "refs/stash")
		if newStash != "" && newStash != oldStash {
			snapshot.stash = newStash
			// Anchor the snapshot independently of stash numbering and reflog expiry.
			if _, err := sourceGit(context.WithoutCancel(ctx), dir, "update-ref", snapshot.backup+"/stash", newStash, ""); err != nil {
				return result, &sourceRecoveryError{fmt.Errorf("changes saved in stash %s, but recovery ref failed: %w", newStash, err)}
			}
			result.Detail += "; saved changes: " + snapshot.backup + "/stash"
		}
		if stashErr != nil {
			return rollbackSourceUpdate(ctx, dir, snapshot, result, stashErr, false)
		}
		remaining, err := sourceGit(ctx, dir, "status", "--porcelain", "--untracked-files=all", "--ignore-submodules=none")
		if err != nil {
			return rollbackSourceUpdate(ctx, dir, snapshot, result, err, false)
		}
		if remaining != "" {
			// A superproject stash cannot save dirty nested worktrees. Never
			// reset it in that state; process the nested checkouts separately.
			return rollbackSourceUpdate(ctx, dir, snapshot, result, fmt.Errorf("checkout remains dirty (possibly a submodule); refusing to reset it"), false)
		}
	}
	if options.Mode == "stash" && snapshot.stash != "" {
		if err := checkSourceStash(ctx, dir, target, snapshot.stash); err != nil {
			return rollbackSourceUpdate(ctx, dir, snapshot, result, fmt.Errorf("saved changes do not apply to the updated checkout: %w", err), false)
		}
	}
	if options.Mode == "mop" {
		if snapshot.targetExisted {
			_, err = sourceGit(ctx, dir, "-c", "submodule.recurse=false", "checkout", "--no-recurse-submodules", snapshot.targetBranch)
		} else {
			_, err = sourceGit(ctx, dir, "-c", "submodule.recurse=false", "checkout", "--no-recurse-submodules", "-b", snapshot.targetBranch, "--track", result.Upstream)
		}
		if err == nil {
			_, err = sourceGit(ctx, dir, "-c", "submodule.recurse=false", "reset", "--hard", target)
		}
	} else {
		_, err = sourceGit(ctx, dir, "-c", "submodule.recurse=false", "merge", "--ff-only", "--no-edit", target)
		if err == nil && snapshot.stash != "" {
			_, err = sourceGit(ctx, dir, "stash", "apply", "--index", snapshot.stash)
		}
	}
	if err != nil {
		return rollbackSourceUpdate(ctx, dir, snapshot, result, err, true)
	}
	result.Status = "updated-" + options.Mode
	if options.Mode == "stash" && before == target {
		result.Status = "unchanged"
	}
	if options.Mode == "mop" {
		result.Branch = snapshot.targetBranch
	}
	return result, nil
}

// Check a stash without exposing conflicts to the user's actual checkout.
func checkSourceStash(ctx context.Context, dir, target, stash string) error {
	temp, err := os.MkdirTemp("", "mwcli-update-code-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	if _, err := sourceGit(ctx, dir, "-c", "submodule.recurse=false", "worktree", "add", "--detach", temp, target); err != nil {
		return err
	}
	defer sourceGit(context.WithoutCancel(ctx), dir, "worktree", "remove", "--force", temp)
	_, err = sourceGit(ctx, temp, "stash", "apply", "--index", stash)
	return err
}

func rollbackSourceUpdate(ctx context.Context, dir string, snapshot sourceSnapshot, result SourceUpdate, cause error, moved bool) (SourceUpdate, error) {
	// Cancellation must not interrupt restoration after changes have been saved.
	recovery, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
	defer cancel()
	run := func(args ...string) error { _, err := sourceGit(recovery, dir, args...); return err }
	var failures []error
	if moved {
		if err := run("-c", "submodule.recurse=false", "reset", "--hard"); err != nil {
			failures = append(failures, err)
		}
		args := []string{"-c", "submodule.recurse=false", "checkout", "--no-recurse-submodules"}
		if snapshot.branch == "" {
			args = append(args, "--detach", snapshot.head)
		} else {
			args = append(args, snapshot.branch)
		}
		if err := run(args...); err != nil {
			failures = append(failures, err)
		} else if err := run("-c", "submodule.recurse=false", "reset", "--hard", snapshot.head); err != nil {
			failures = append(failures, err)
		}
		if len(failures) == 0 && snapshot.targetBranch != "" && snapshot.targetBranch != snapshot.branch {
			if snapshot.targetExisted {
				if err := run("update-ref", "refs/heads/"+snapshot.targetBranch, snapshot.targetHead); err != nil {
					failures = append(failures, err)
				}
			} else if err := run("branch", "-D", snapshot.targetBranch); err != nil {
				failures = append(failures, err)
			}
		}
	}
	if snapshot.stash != "" && len(failures) == 0 {
		if !moved {
			// The original HEAD and index have not moved. Applying the saved
			// snapshot here restores the worktree without resetting submodules.
			if err := run("stash", "apply", "--index", snapshot.stash); err != nil {
				failures = append(failures, err)
			}
		} else {
			// Restore the saved worktree and index separately. Unlike popping a
			// stash, this also handles untracked files left by a failed apply.
			if err := run("restore", "--source="+snapshot.stash, "--worktree", "--", "."); err != nil {
				failures = append(failures, err)
			}
			if err := run("read-tree", snapshot.stash+"^2"); err != nil {
				failures = append(failures, err)
			}
			if files, err := sourceGit(recovery, dir, "ls-tree", "--name-only", snapshot.stash+"^3"); err == nil && files != "" {
				if err := run("restore", "--source="+snapshot.stash+"^3", "--worktree", "--overlay", "--", "."); err != nil {
					failures = append(failures, err)
				}
			}
		}
	}
	if len(failures) != 0 {
		result.Status = "recovery-needed"
		return result, &sourceRecoveryError{fmt.Errorf("update failed: %w; automatic restoration failed: %w; use recovery refs under %s", cause, errors.Join(failures...), snapshot.backup)}
	}
	result.Status = "restored"
	return result, fmt.Errorf("update was not applied; original checkout restored: %w", cause)
}

// Ignored files are not part of a stash. Refuse updates whose tracked paths
// would overwrite those files instead of silently losing local ignored data.
func sourceIgnoredCollisions(ctx context.Context, dir, target string) error {
	ignoredOutput, err := sourceGitOutput(ctx, dir, "ls-files", "--others", "--ignored", "--exclude-standard", "-z")
	if err != nil {
		return err
	}
	if ignoredOutput == "" {
		return nil
	}
	trackedOutput, err := sourceGitOutput(ctx, dir, "ls-tree", "-rz", "--name-only", target)
	if err != nil {
		return err
	}
	ignored := make(map[string]bool)
	tracked := make(map[string]bool)
	for _, path := range strings.Split(ignoredOutput, "\x00") {
		if path != "" {
			ignored[strings.TrimSuffix(path, "/")] = true
		}
	}
	for _, path := range strings.Split(trackedOutput, "\x00") {
		if path == "" {
			continue
		}
		tracked[path] = true
		for part := path; part != "."; part = filepath.ToSlash(filepath.Dir(part)) {
			if ignored[part] {
				return fmt.Errorf("updating would overwrite ignored path %q; move it aside first", part)
			}
		}
	}
	for path := range ignored {
		for part := path; part != "."; part = filepath.ToSlash(filepath.Dir(part)) {
			if tracked[part] {
				return fmt.Errorf("updating would overwrite ignored path %q; move it aside first", path)
			}
		}
	}
	return nil
}
