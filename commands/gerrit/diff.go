package gerrit

import (
	"context"
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	gogerrit "github.com/andygrunwald/go-gerrit"
	"github.com/spf13/cobra"
	gerritapi "gitlab.wikimedia.org/repos/releng/cli/internal/gerrit"
)

func newDiffCmd() *cobra.Command {
	var patchset, base int
	cmd := &cobra.Command{
		Use:     "diff [change]",
		Short:   "Show a patchset diff or compare two Gerrit patchsets",
		Long:    "Show a public patchset against its first parent, or compare the trees of two patchsets with --base-patchset. Pass a change number or URL, or use the uploaded patchset matching Git HEAD. Git fetches into a temporary repository.",
		Example: "diff\ndiff 1370181 --base-patchset 3 --patchset 4",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("base-patchset") && base < 1 {
				return fmt.Errorf("--base-patchset must be positive")
			}
			client, target, err := preparePublicChange(cmd.Context(), cmd, args, patchset)
			if err != nil {
				return err
			}
			fields := []string{}
			if base != 0 {
				fields = append(fields, "ALL_REVISIONS")
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), gerritRequestTimeout)
			defer cancel()
			change, selected, err := target.readChange(ctx, client, fields...)
			if err != nil {
				return err
			}
			return diffGerritPatchsets(cmd, target.change, change, selected, base)
		},
	}
	cmd.Flags().IntVar(&base, "base-patchset", 0, "Compare against this patchset of the same change")
	return publicPatchsetCommand(cmd, &patchset)
}

func diffGerritPatchsets(cmd *cobra.Command, number int, change *gogerrit.ChangeInfo, selected, base int) error {
	commit, revision, ok := gerritapi.Revision(change, selected)
	if !ok {
		return fmt.Errorf("patchset %d is unavailable for Gerrit change %d", selected, number)
	}
	address, ref, err := verifiedRevisionFetch(change.Project, number, commit, revision)
	if err != nil {
		return err
	}
	var baseCommit string
	refspecs := []string{ref + ":refs/mw/selected"}
	if base != 0 && base != selected {
		baseSHA, baseRevision, ok := gerritapi.Revision(change, base)
		if !ok {
			return fmt.Errorf("base patchset %d is unavailable for Gerrit change %d", base, number)
		}
		baseURL, baseRef, err := verifiedRevisionFetch(change.Project, number, baseSHA, baseRevision)
		if err != nil {
			return err
		}
		if baseURL != address {
			return fmt.Errorf("Gerrit returned different fetch URLs for the two patchsets")
		}
		baseCommit = baseSHA
		refspecs = append(refspecs, baseRef+":refs/mw/base")
	} else if base == selected {
		return nil
	}

	directory, err := os.MkdirTemp("", "mw-gerrit-diff-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(directory)

	ctx, cancel := context.WithTimeout(cmd.Context(), time.Minute)
	defer cancel()
	git := func(args ...string) *osexec.Cmd {
		options := []string{
			"--git-dir", directory, "-c", "core.hooksPath=" + filepath.Join(directory, "no-hooks"),
			"-c", "credential.helper=", "-c", "core.askPass=",
		}
		process := gerritGit(ctx, directory, append(options, args...)...)
		// Isolate repo storage even when invoked from a Git hook or linked worktree.
		process.Env = slices.DeleteFunc(process.Env, func(entry string) bool {
			key, _, _ := strings.Cut(entry, "=")
			switch strings.ToUpper(key) {
			case "GIT_DIR", "GIT_COMMON_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_SHALLOW_FILE", "GIT_GRAFT_FILE":
				return true
			}
			return false
		})
		process.Env = append(process.Env, "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=")
		return process
	}
	if result, err := git("init", "--bare", "--quiet", "--template=").CombinedOutput(); err != nil {
		return fmt.Errorf("initialise temporary diff repository: %s: %w", strings.TrimSpace(string(result)), err)
	}
	fetchArgs := []string{"fetch", "--quiet", "--depth=2", "--no-tags", "--no-recurse-submodules", "--no-write-fetch-head", "--no-auto-maintenance", "--", address}
	if result, err := git(append(fetchArgs, refspecs...)...).CombinedOutput(); err != nil {
		return fmt.Errorf("fetch Gerrit patchsets for diff: %s: %w", strings.TrimSpace(string(result)), err)
	}

	for ref, want := range map[string]string{"refs/mw/selected": commit, "refs/mw/base": baseCommit} {
		if want == "" {
			continue
		}
		actual, err := git("rev-parse", "--verify", ref).Output()
		if err != nil || strings.TrimSpace(string(actual)) != want {
			return fmt.Errorf("fetched Gerrit patchset does not match its advertised commit")
		}
	}

	if baseCommit == "" {
		data, err := git("show", "-s", "--format=%P", commit).Output()
		if err != nil {
			return fmt.Errorf("read Gerrit patchset parent: %w", err)
		}
		if parents := strings.Fields(string(data)); len(parents) != 0 {
			baseCommit = parents[0]
		}
	}
	diffOptions := []string{"--no-color", "--no-ext-diff", "--no-textconv", "--binary", "--find-renames"}
	args := append([]string{"diff"}, diffOptions...)
	args = append(args, baseCommit, commit, "--")
	if baseCommit == "" {
		args = append([]string{"diff-tree", "--root", "--no-commit-id", "-r", "-p"}, diffOptions...)
		args = append(args, commit, "--")
	}
	diff := git(args...)
	diff.Stdout, diff.Stderr = cmd.OutOrStdout(), cmd.ErrOrStderr()
	if err := diff.Run(); err != nil {
		return fmt.Errorf("compare Gerrit patchsets: %w", err)
	}
	return nil
}
