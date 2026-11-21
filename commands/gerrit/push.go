package gerrit

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	osexec "os/exec"
	"regexp"
	"strconv"
	"strings"

	gogerrit "github.com/andygrunwald/go-gerrit"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/gitconfig"
)

var changeIDPattern = regexp.MustCompile(`^I[0-9a-fA-F]{40}$`)

func newPushCmd() *cobra.Command {
	var dryRun, https bool
	var branch, remote string
	cmd := &cobra.Command{
		Use:     "push",
		Short:   "Upload commits from this Git checkout for Gerrit review",
		Long:    "Push the current commit stack to refs/for/<branch>. Use --dry-run to show the outgoing commits and matching public reviews without uploading. --https uses the selected Gerrit profile’s saved HTTP password.",
		Example: "push --dry-run\npush --remote origin --branch master\npush --https",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			plan, err := planGerritPush(cmd.Context(), remote, branch, https)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Remote %s: %s\nTarget: refs/for/%s\n", plan.remote, gerritSetupURL(plan.address), plan.branch)
			if dryRun {
				return previewGerritPush(cmd, plan)
			}
			return uploadGerritPush(cmd, plan)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview the upload without pushing")
	cmd.Flags().BoolVar(&https, "https", false, "Use HTTPS and the selected profile’s saved HTTP password")
	cmd.Flags().StringVar(&branch, "branch", "", "Target branch in Gerrit")
	cmd.Flags().StringVar(&remote, "remote", "", "Git remote to use (default: Git's push remote)")
	return commandmeta.Set(cmd, commandmeta.Hints{
		ToolAnnotations: commandmeta.ToolAnnotations{DestructiveHint: true, OpenWorldHint: true},
		DataAccess:      []commandmeta.DataAccess{commandmeta.DataPublic, commandmeta.DataPrivateMetadata, commandmeta.DataSuppliedContent, commandmeta.DataSecrets},
	})
}

type gerritPushPlan struct {
	remote, address, project, head, branch, tip string
}

func planGerritPush(ctx context.Context, remote, branch string, forceHTTPS bool) (gerritPushPlan, error) {
	var plan gerritPushPlan
	head, err := gitOutput(ctx, "", "rev-parse", "--verify", "HEAD")
	if err != nil {
		return plan, fmt.Errorf("cannot read Git HEAD; run inside a checkout with at least one commit")
	}
	localBranch, _ := gitOutput(ctx, "", "symbolic-ref", "--quiet", "--short", "HEAD")
	if remote == "" {
		remote = gitconfig.PushRemote(ctx, localBranch)
	}
	address, err := gitOutput(ctx, "", "remote", "get-url", "--push", "--all", "--", remote)
	if err != nil || address == "" {
		return plan, fmt.Errorf("could not read push URL for Git remote %q; select one with --remote", remote)
	}
	if strings.Contains(address, "\n") {
		return plan, fmt.Errorf("Git remote %q has multiple push URLs; choose a remote with one destination", remote)
	}
	destination := gerritSetupURL(address)
	project := gerritRemoteProject(destination)
	if project == "" || (destination.Scheme != "https" && destination.Scheme != "ssh") {
		return plan, fmt.Errorf("Git remote %q has no Wikimedia Gerrit SSH or HTTPS push URL; use mw gerrit setup --push-protocol", remote)
	}

	client, err := newGerritClient(ctx)
	if err != nil {
		return plan, err
	}
	if branch == "" {
		branch, err = inferredReviewBranch(ctx, client, project, head, localBranch)
		if err != nil {
			return plan, err
		}
	}
	fetchURL := gerritPushURL(project, "https", "")
	branch, tip, err := gerritBranchTip(ctx, fetchURL, branch)
	if err != nil {
		return plan, err
	}
	if strings.Contains(branch, "%") {
		return plan, fmt.Errorf("Gerrit target branch may not contain %%")
	}
	if forceHTTPS || destination.Scheme == "https" {
		// Use the standard Wikimedia HTTPS endpoint and keep credentials out of URLs.
		address = gerritPushURL(project, "https", "")
	}
	return gerritPushPlan{remote: remote, address: address, project: project, head: head, branch: branch, tip: tip}, nil
}

func previewGerritPush(cmd *cobra.Command, plan gerritPushPlan) error {
	ctx := cmd.Context()
	commits, err := outgoingGerritCommits(ctx, gerritPushURL(plan.project, "https", ""), plan.branch, plan.tip, plan.head)
	if err != nil {
		return err
	}
	if len(commits) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No outgoing commits")
		return nil
	}
	client, err := newGerritClient(ctx)
	if err != nil {
		return err
	}
	for _, commit := range commits {
		if err := printGerritPushCommit(cmd, client, plan.project, plan.branch, commit); err != nil {
			return err
		}
	}
	return nil
}

func inferredReviewBranch(ctx context.Context, client *gogerrit.Client, project, head, localBranch string) (string, error) {
	if localBranch != "" {
		return trackedReviewBranch(ctx, project, localBranch)
	}

	changes, err := queryPushChanges(ctx, client, "project:"+strconv.Quote(project)+" commit:"+head)
	if err != nil {
		return "", fmt.Errorf("look up detached HEAD in Gerrit: %w", err)
	}
	if branch := uniqueReviewBranch(changes, project); branch != "" {
		return branch, nil
	}
	if len(changes) == 0 {
		trailers, _ := gitOutput(ctx, "", "show", "-s", "--format=%(trailers:only,unfold)", head)
		if id := gerritTrailerID(trailers); id != "" {
			changes, err := queryPushChanges(ctx, client, "project:"+strconv.Quote(project)+" status:open change:"+id)
			if err != nil {
				return "", fmt.Errorf("look up the detached commit’s Change-Id in Gerrit: %w", err)
			}
			if branch := uniqueReviewBranch(changes, project); branch != "" {
				return branch, nil
			}
		}
	}
	return "", fmt.Errorf("detached HEAD did not match one public Gerrit change in %s; specify --branch", project)
}

func trackedReviewBranch(ctx context.Context, project, localBranch string) (string, error) {
	prefix := "branch." + localBranch
	upstreamRemote, _ := gitOutput(ctx, "", "config", "--get", prefix+".remote")
	mergeRef, _ := gitOutput(ctx, "", "config", "--get-all", prefix+".merge")
	if upstreamRemote == "" && mergeRef == "" {
		return "", nil // Use the Gerrit project's advertised default branch.
	}
	branch, ok := strings.CutPrefix(mergeRef, "refs/heads/")
	if !ok || branch == "" || strings.Contains(branch, "\n") || upstreamRemote == "" || upstreamRemote == "." {
		return "", fmt.Errorf("cannot infer one Gerrit target from the upstream of %q; specify --branch", localBranch)
	}
	fetchURL, err := gitOutput(ctx, "", "remote", "get-url", "--", upstreamRemote)
	if err != nil || (gerritRemoteProject(gerritSetupURL(fetchURL)) != project && !checkoutRemoteMatches(fetchURL, project, true)) {
		return "", fmt.Errorf("cannot confirm that upstream remote %q fetches Gerrit project %s; specify --branch", upstreamRemote, project)
	}
	return branch, nil
}

func uniqueReviewBranch(changes []gogerrit.ChangeInfo, project string) string {
	if len(changes) == 1 && !changes[0].MoreChanges && changes[0].Project == project {
		return changes[0].Branch
	}
	return ""
}

func gerritBranchTip(ctx context.Context, fetchURL, branch string) (string, string, error) {
	ref := "HEAD"
	if branch != "" {
		branch = strings.TrimPrefix(branch, "refs/heads/")
		ref = "refs/heads/" + branch
		if _, err := gitOutput(ctx, "", "check-ref-format", ref); err != nil {
			return "", "", fmt.Errorf("invalid Gerrit target branch %q", branch)
		}
	}
	data, err := gerritReadGit(ctx, "ls-remote", "--symref", "--exit-code", "--", fetchURL, ref)
	if err != nil {
		return "", "", fmt.Errorf("read Gerrit ref %s: %w", ref, err)
	}
	var tip string
	for line := range strings.SplitSeq(data, "\n") {
		value, name, _ := strings.Cut(line, "\t")
		if name != ref {
			continue
		}
		if target, ok := strings.CutPrefix(value, "ref: refs/heads/"); ok && ref == "HEAD" {
			branch = target
		} else if decoded, err := hex.DecodeString(value); err == nil && len(decoded) == 20 {
			tip = value
		}
	}
	if branch == "" || tip == "" {
		return "", "", fmt.Errorf("could not resolve %s to a Gerrit branch and tip; specify --branch", ref)
	}
	return branch, tip, nil
}

func gerritReadGit(ctx context.Context, args ...string) (string, error) {
	git := gerritGit(ctx, "", args...)
	git.Env = append(git.Env, "GIT_TERMINAL_PROMPT=0")
	data, err := git.Output()
	if failure, ok := err.(*osexec.ExitError); ok && len(failure.Stderr) != 0 {
		err = fmt.Errorf("%s: %w", strings.TrimSpace(string(failure.Stderr)), err)
	}
	return strings.TrimSpace(string(data)), err
}

func outgoingGerritCommits(ctx context.Context, fetchURL, branch, tip, head string) ([]string, error) {
	if shallow, err := gitOutput(ctx, "", "rev-parse", "--is-shallow-repository"); err != nil || shallow != "false" {
		return nil, fmt.Errorf("a complete Git history is needed to preview outgoing commits; use a full clone")
	}
	list := gerritGit(ctx, "", "rev-list", "--reverse", "--topo-order", tip+".."+head, "--")
	if _, err := gitOutput(ctx, "", "cat-file", "-e", tip+"^{commit}"); err != nil {
		objects, err := gitOutput(ctx, "", "rev-parse", "--path-format=absolute", "--git-path", "objects")
		if err != nil {
			return nil, fmt.Errorf("locate Git object store: %w", err)
		}
		temp, err := os.MkdirTemp("", "mw-gerrit-preview-")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(temp)
		// Read local objects as alternates and keep newly fetched objects outside the checkout.
		alternates := strconv.Quote(objects)
		if inherited := os.Getenv("GIT_ALTERNATE_OBJECT_DIRECTORIES"); inherited != "" {
			alternates += string(os.PathListSeparator) + inherited
		}
		env := []string{"GIT_OBJECT_DIRECTORY=" + temp, "GIT_ALTERNATE_OBJECT_DIRECTORIES=" + alternates, "GIT_TERMINAL_PROMPT=0"}
		fetch := gerritGit(ctx, "", "-c", "fetch.writeCommitGraph=false", "fetch",
			"--no-tags", "--no-write-fetch-head", "--no-auto-maintenance", "--no-recurse-submodules", "--no-prune",
			"--refmap=", "--", fetchURL, "refs/heads/"+branch)
		fetch.Env = append(fetch.Env, env...)
		if data, err := fetch.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("fetch Gerrit branch for preview: %s: %w", strings.TrimSpace(string(data)), err)
		}
		list.Env = append(list.Env, env...)
	}
	output, err := list.Output()
	if err != nil {
		return nil, fmt.Errorf("compare HEAD with Gerrit branch %s: %w", branch, err)
	}
	return strings.Fields(string(output)), nil
}

func printGerritPushCommit(cmd *cobra.Command, client *gogerrit.Client, project, branch, commit string) error {
	ctx := cmd.Context()
	data, err := gitOutput(ctx, "", "show", "-s", "--format=%s%n%(trailers:only,unfold)", commit)
	if err != nil {
		return fmt.Errorf("read commit %s: %w", commit, err)
	}
	subject, trailers, _ := strings.Cut(data, "\n")
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "\n%s  %s\n", commit[:12], subject)
	id := gerritTrailerID(trailers)
	if id == "" {
		fmt.Fprintln(out, "  Missing Change-Id. Run mw gerrit setup --install-hook, then amend the commit.")
		return nil
	}
	fmt.Fprintf(out, "  Change-Id: %s\n", id)
	changes, err := queryPushChanges(ctx, client, "project:"+strconv.Quote(project)+" branch:"+strconv.Quote(branch)+" change:"+id)
	if err != nil {
		return fmt.Errorf("look up review for %s: %w", id, err)
	}
	if len(changes) == 0 {
		fmt.Fprintln(out, "  No matching public review found")
		return nil
	}
	if len(changes) != 1 || changes[0].MoreChanges {
		fmt.Fprintln(out, "  Multiple Gerrit reviews match this Change-Id")
		return nil
	}
	change := changes[0]
	link := fmt.Sprintf("%s/%d", secrets.GerritEndpoint, change.Number)
	switch change.Status {
	case "NEW":
		if change.CurrentRevision == commit {
			fmt.Fprintf(out, "  Already the current patchset on review %d: %s\n", change.Number, link)
		} else {
			fmt.Fprintf(out, "  Would update review %d: %s\n", change.Number, link)
		}
	case "MERGED", "ABANDONED":
		fmt.Fprintf(out, "  Review %d is %s: %s\n", change.Number, strings.ToLower(change.Status), link)
	default:
		fmt.Fprintf(out, "  Review %d: %s\n", change.Number, link)
	}
	return nil
}

func gerritTrailerID(trailers string) string {
	const linkPrefix = secrets.GerritEndpoint + "/id/"
	var linked string
	for line := range strings.SplitSeq(trailers, "\n") {
		key, value, _ := strings.Cut(line, ":")
		value = strings.TrimSpace(value)
		if strings.EqualFold(key, "Change-Id") && changeIDPattern.MatchString(value) {
			return value
		}
		if strings.EqualFold(key, "Link") {
			if id, ok := strings.CutPrefix(value, linkPrefix); ok && changeIDPattern.MatchString(id) {
				linked = id
			}
		}
	}
	return linked
}

func queryPushChanges(ctx context.Context, client *gogerrit.Client, query string) ([]gogerrit.ChangeInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, gerritRequestTimeout)
	defer cancel()
	changes, resp, err := client.Changes.QueryChanges(ctx, &gogerrit.QueryChangeOptions{
		QueryOptions:  gogerrit.QueryOptions{Query: []string{query}, Limit: 2},
		ChangeOptions: gogerrit.ChangeOptions{AdditionalFields: []string{"CURRENT_REVISION"}},
	})
	if err != nil {
		if resp != nil {
			resp.Body.Close()
		}
		return nil, err
	}
	return *changes, nil
}

func uploadGerritPush(cmd *cobra.Command, plan gerritPushPlan) error {
	ctx := cmd.Context()
	var options []string
	isHTTPS := strings.HasPrefix(plan.address, "https::")
	if isHTTPS {
		var err error
		options, err = gerritHTTPSOptions(cmd, strings.TrimPrefix(plan.address, "https::"))
		if err != nil {
			return err
		}
	}
	effective, err := gitOutput(ctx, "", "ls-remote", "--get-url", "--", plan.address)
	if err != nil || effective != plan.address {
		return fmt.Errorf("Git would rewrite the resolved push URL again; check your Git URL rewrites or select another remote")
	}
	args := append(options, "-c", "push.pushOption=", "push", "--no-follow-tags", "--no-mirror", "--recurse-submodules=no",
		"--", plan.address, plan.head+":refs/for/"+plan.branch)
	git := gerritGit(ctx, "", args...)
	git.Stdin, git.Stdout, git.Stderr = cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr()
	if isHTTPS {
		git.Env = gerritHTTPSPushEnv(git.Env)
	}
	if err := git.Run(); err != nil {
		return fmt.Errorf("upload Gerrit review: %w", err)
	}
	return nil
}
