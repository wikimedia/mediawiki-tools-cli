package mediawiki

import (
	"context"
	"errors"
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
)

type (
	SourceRepository struct{ Name, Directory string }
	SourceUpdate     struct{ Name, Branch, Upstream, Status, Detail string }
)

// SourceRepositories lists independent checkouts and Git submodules/worktrees;
// plain extension directories are not treated as the enclosing core repository.
func (m MediaWiki) SourceRepositories() ([]SourceRepository, error) {
	if _, err := os.Stat(filepath.Join(m.Directory(), ".git")); err != nil {
		return nil, fmt.Errorf("core is not a Git checkout: %w", err)
	}
	repos := []SourceRepository{{Name: "core", Directory: m.Directory()}}
	for _, group := range []string{"skins", "extensions"} {
		entries, err := os.ReadDir(m.Path(group))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			directory := m.Path(filepath.Join(group, entry.Name()))
			if _, err := os.Stat(filepath.Join(directory, ".git")); err == nil {
				repos = append(repos, SourceRepository{Name: group + "/" + entry.Name(), Directory: directory})
			}
		}
	}
	return repos, nil
}

func sourceGit(ctx context.Context, directory string, args ...string) (string, error) {
	cmd := osexec.CommandContext(ctx, "git", append([]string{"-C", directory}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

// UpdateSourceRepositories never resets, checks out branches, rebases or creates
// merge commits. A dry run does not fetch or otherwise contact remotes.
func UpdateSourceRepositories(ctx context.Context, repos []SourceRepository, dryRun bool) ([]SourceUpdate, error) {
	results := make([]SourceUpdate, 0, len(repos))
	var failures []error
	for _, repo := range repos {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		result, err := updateSourceRepository(ctx, repo, dryRun)
		results = append(results, result)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", repo.Name, err))
		}
	}
	return results, errors.Join(failures...)
}

func updateSourceRepository(ctx context.Context, repo SourceRepository, dryRun bool) (SourceUpdate, error) {
	result := SourceUpdate{Name: repo.Name, Status: "failed"}
	dirty, err := sourceGit(ctx, repo.Directory, "status", "--porcelain", "--untracked-files=normal")
	if err != nil {
		return result, err
	}
	if dirty != "" {
		result.Status = "skipped-dirty"
		return result, nil
	}
	branch, err := sourceGit(ctx, repo.Directory, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		var exitErr *osexec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			result.Status = "skipped-detached"
			return result, nil
		}
		return result, err
	}
	result.Branch = branch
	upstream, err := sourceGit(ctx, repo.Directory, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if err != nil {
		result.Status = "skipped-no-upstream"
		return result, nil
	}
	result.Upstream = upstream
	if dryRun {
		result.Status = "would-pull-ff-only"
		return result, nil
	}
	before, err := sourceGit(ctx, repo.Directory, "rev-parse", "HEAD")
	if err != nil {
		return result, err
	}
	detail, err := sourceGit(ctx, repo.Directory, "pull", "--ff-only", "--no-rebase", "--no-recurse-submodules")
	if err != nil {
		return result, err
	}
	after, err := sourceGit(ctx, repo.Directory, "rev-parse", "HEAD")
	if err != nil {
		return result, err
	}
	result.Status = "unchanged"
	if before != after {
		result.Status = "updated"
	}
	result.Detail = detail
	return result, nil
}
