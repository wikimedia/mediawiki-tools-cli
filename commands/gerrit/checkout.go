package gerrit

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/url"
	osexec "os/exec"
	"strconv"
	"strings"

	gogerrit "github.com/andygrunwald/go-gerrit"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/accesspolicy"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	gerritapi "gitlab.wikimedia.org/repos/releng/cli/internal/gerrit"
)

func newCheckoutCmd() *cobra.Command {
	var patchset int
	cmd := &cobra.Command{
		Use:   "checkout <change>",
		Short: "Check out a Gerrit change in the current Git repository",
		Long: "Use a public Wikimedia Gerrit change number or URL. Fetch the latest patchset, or select one with --patchset. " +
			"The current repository must have a matching remote and a clean worktree. The checkout detaches HEAD, leaving local branches in place.",
		Example:           "checkout 1370181\ncheckout 1370181 --patchset 4",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("patchset") && patchset < 1 {
				return fmt.Errorf("--patchset must be positive")
			}
			change, selected, err := parseGerritChange(args[0], patchset)
			if err != nil {
				return err
			}
			return checkoutChange(cmd, change, selected)
		},
	}
	cmd.Flags().IntVar(&patchset, "patchset", 0, "Check out this patchset")
	accesspolicy.Support(cmd)
	return commandmeta.Set(cmd, commandmeta.Hints{
		ToolAnnotations: commandmeta.ToolAnnotations{IdempotentHint: true, OpenWorldHint: true},
		DataAccess:      []commandmeta.DataAccess{commandmeta.DataPublic},
	})
}

func checkoutChange(cmd *cobra.Command, number, patchset int) error {
	ctx := cmd.Context()
	status, err := osexec.CommandContext(ctx, "git", "status", "--porcelain", "--untracked-files=normal").Output()
	if err != nil {
		return fmt.Errorf("check Git worktree: %w. Run this command inside the change's repository", err)
	}
	if len(status) != 0 {
		return fmt.Errorf("Git worktree has uncommitted changes. Commit or stash them before checking out a Gerrit change")
	}

	change, err := checkoutChangeInfo(ctx, number, patchset)
	if err != nil {
		return err
	}
	commit, revision, ok := gerritapi.Revision(change, patchset)
	if !ok {
		if patchset == 0 {
			return fmt.Errorf("current patchset is unavailable for Gerrit change %d", number)
		}
		return fmt.Errorf("patchset %d is unavailable for Gerrit change %d", patchset, number)
	}
	if err := verifyCheckoutRepository(ctx, change.Project, number); err != nil {
		return err
	}
	address, ref, err := verifiedRevisionFetch(change.Project, number, commit, revision)
	if err != nil {
		return err
	}

	runGit := func(args ...string) error {
		git := osexec.CommandContext(ctx, "git", args...)
		git.Stdin, git.Stdout, git.Stderr = cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr()
		return git.Run()
	}
	// Select the HTTPS transport explicitly so an HTTPS-to-SSH insteadOf rule
	// cannot make an anonymous checkout require SSH credentials.
	if err := runGit("fetch", "--no-tags", "--no-recurse-submodules", "--no-write-fetch-head", "--", address, ref); err != nil {
		return fmt.Errorf("fetch Gerrit change %d, patchset %d: %w", number, revision.Number, err)
	}
	// Use the selected commit so a concurrent fetch cannot redirect this checkout.
	if err := runGit("checkout", "--detach", "--no-recurse-submodules", "--no-overwrite-ignore", commit); err != nil {
		return fmt.Errorf("check out Gerrit change %d, patchset %d: %w", number, revision.Number, err)
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Checked out change %d, patchset %d (%s)\n", number, revision.Number, change.Project)
	return err
}

func verifiedRevisionFetch(project string, number int, commit string, revision gogerrit.RevisionInfo) (string, string, error) {
	address, ref := gerritapi.RevisionFetch(project, revision)
	expectedRef := fmt.Sprintf("refs/changes/%02d/%d/%d", number%100, number, revision.Number)
	if _, err := hex.DecodeString(commit); err != nil || len(commit) != 40 || ref != expectedRef ||
		!checkoutRemoteMatches(address, project, false) {
		return "", "", fmt.Errorf("Gerrit returned unexpected fetch details for change %d, patchset %d", number, revision.Number)
	}
	return "https::" + address, ref, nil
}

func checkoutChangeInfo(ctx context.Context, number, patchset int) (*gogerrit.ChangeInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, gerritRequestTimeout)
	defer cancel()
	client, err := newGerritClient(ctx)
	if err != nil {
		return nil, err
	}
	field := "CURRENT_REVISION"
	if patchset != 0 {
		field = "ALL_REVISIONS"
	}
	change, _, err := client.Changes.GetChange(ctx, strconv.Itoa(number), &gogerrit.ChangeOptions{
		AdditionalFields: []string{field},
	})
	if err != nil {
		return nil, fmt.Errorf("read Gerrit change %d: %w", number, err)
	}
	return change, nil
}

func verifyCheckoutRepository(ctx context.Context, project string, number int) error {
	// Read the configured URLs before Git applies any insteadOf rewrites.
	git := osexec.CommandContext(ctx, "git", "config", "--get-regexp", `^remote\..*\.(url|pushurl)$`)
	remotes, err := git.Output()
	if err != nil {
		if exit, ok := err.(*osexec.ExitError); !ok || exit.ExitCode() != 1 {
			return fmt.Errorf("read Git remotes: %w", err)
		}
	}
	for line := range strings.SplitSeq(string(remotes), "\n") {
		_, address, _ := strings.Cut(line, " ")
		if checkoutRemoteMatches(address, project, true) {
			return nil
		}
	}
	message := fmt.Sprintf("change belongs to %s, but the current Git repository has no matching remote. Run this command in that repository", project)
	if project == "mediawiki/core" || strings.HasPrefix(project, "mediawiki/extensions/") || strings.HasPrefix(project, "mediawiki/skins/") {
		message += fmt.Sprintf(", or run mw dev mediawiki apply-patches --change %d --clone-missing", number)
	}
	return fmt.Errorf("%s", message)
}

func checkoutRemoteMatches(address, project string, localRemote bool) bool {
	// Clones configured for HTTPS use Git’s explicit transport syntax.
	if localRemote && strings.HasPrefix(address, "https::https://") {
		address = strings.TrimPrefix(address, "https::")
	}
	if host, path, ok := strings.Cut(address, ":"); localRemote && ok && !strings.Contains(address, "://") {
		host = host[strings.LastIndexByte(host, '@')+1:]
		if host == "gerrit.wikimedia.org" || host == "github.com" {
			address = "ssh://" + host + "/" + path
		}
	}
	remote, err := url.Parse(address)
	if err != nil || remote.Opaque != "" {
		return false
	}
	if !localRemote && (remote.Scheme != "https" || remote.Host != "gerrit.wikimedia.org" ||
		remote.User != nil || remote.RawQuery != "" || remote.Fragment != "") {
		return false
	}
	path := strings.TrimSuffix(strings.Trim(remote.Path, "/"), ".git")
	switch remote.Hostname() {
	case "gerrit.wikimedia.org":
		switch remote.Scheme {
		case "https", "http":
			return strings.TrimPrefix(path, "r/") == project && project != ""
		case "ssh":
			return path == project && project != ""
		}
	case "github.com":
		if remote.Scheme != "https" && remote.Scheme != "ssh" {
			return false
		}
		if project == "mediawiki/core" {
			return strings.EqualFold(path, "wikimedia/mediawiki")
		}
		if strings.HasPrefix(project, "mediawiki/extensions/") || strings.HasPrefix(project, "mediawiki/skins/") {
			return strings.EqualFold(path, "wikimedia/"+strings.ReplaceAll(project, "/", "-"))
		}
	}
	return false
}
