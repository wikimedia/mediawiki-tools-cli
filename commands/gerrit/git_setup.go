package gerrit

import (
	"context"
	"fmt"
	"net/url"
	osexec "os/exec"
	"path"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	gerritapi "gitlab.wikimedia.org/repos/releng/cli/internal/gerrit"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

var projectSegment = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

func validGerritProject(project string) bool {
	for segment := range strings.SplitSeq(project, "/") {
		if !projectSegment.MatchString(segment) || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

func gerritRemoteProject(remote *url.URL) string {
	if remote == nil {
		return ""
	}
	project := strings.Trim(remote.Path, "/")
	if remote.Scheme != "ssh" {
		var ok bool
		project, ok = strings.CutPrefix(project, "r/")
		if !ok {
			return ""
		}
		project = strings.TrimPrefix(project, "a/")
	}
	project = strings.TrimSuffix(project, ".git")
	if !validGerritProject(project) {
		return ""
	}
	return project
}

func gerritGit(ctx context.Context, directory string, args ...string) *osexec.Cmd {
	git := osexec.CommandContext(ctx, "git", args...)
	git.Dir = directory
	git.Env = append(git.Environ(), "GIT_HTTP_USER_AGENT="+cli.HTTPUserAgent())
	return git
}

func gitOutput(ctx context.Context, directory string, args ...string) (string, error) {
	result, err := gerritGit(ctx, directory, args...).Output()
	return strings.TrimSpace(string(result)), err
}

func runGerritGit(cmd *cobra.Command, directory string, args ...string) error {
	git := gerritGit(cmd.Context(), directory, args...)
	git.Stdin, git.Stdout, git.Stderr = cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr()
	return git.Run()
}

func gerritPushURL(project, protocol, username string) string {
	if protocol == "https" {
		// Select the HTTPS transport even when the user has an HTTPS-to-SSH insteadOf rule.
		return "https::" + secrets.GerritEndpoint + "/" + project
	}
	remote := url.URL{Scheme: "ssh", Host: gerritSSHHost + ":" + gerritSSHPort, Path: "/" + project}
	if username != "" {
		remote.User = url.User(username)
	}
	return remote.String()
}

func validatePushProtocol(protocol string) error {
	if protocol != "ssh" && protocol != "https" {
		return fmt.Errorf("--push-protocol must be ssh or https")
	}
	return nil
}

func newCloneCmd() *cobra.Command {
	var protocol string
	cmd := &cobra.Command{
		Use:     "clone <project> [directory]",
		Short:   "Clone a Gerrit project and prepare it for review",
		Long:    "Clone a public Wikimedia Gerrit project over HTTPS, install its commit-msg hook, and configure the push URL in the clone. SSH pushes use the selected Gerrit username when available. HTTPS pushes use your Git credentials.",
		Example: "clone mediawiki/extensions/WSOAuth\nclone mediawiki/extensions/WSOAuth --push-protocol https",
		Args:    cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			project := args[0]
			if !validGerritProject(project) {
				return fmt.Errorf("provide a Wikimedia Gerrit project name, such as mediawiki/core")
			}
			if err := validatePushProtocol(protocol); err != nil {
				return err
			}
			username, _, err := gerritIdentity(cmd)
			if err != nil {
				return err
			}
			directory := path.Base(project)
			if len(args) == 2 {
				directory = args[1]
			}
			if err := runGerritGit(cmd, "", "clone", "--", "https::"+secrets.GerritEndpoint+"/"+project, directory); err != nil {
				return fmt.Errorf("clone Gerrit project: %w", err)
			}
			if err := setGerritPushURL(cmd, directory, "origin", project, protocol, username); err != nil {
				return fmt.Errorf("cloned to %s; push URL setup failed: %w", directory, err)
			}
			if err := gerritapi.InstallCommitHook(cmd.Context(), directory, gerritapi.CommitHookURL); err != nil {
				return fmt.Errorf("cloned to %s; run mw gerrit setup --install-hook there to retry: %w", directory, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Cloned %s to %s. Run mw gerrit setup --check inside the checkout.\n", project, directory)
			return nil
		},
	}
	cmd.Flags().StringVar(&protocol, "push-protocol", "ssh", "Git transport for pushes (ssh or https)")
	return commandmeta.Set(cmd, commandmeta.Hints{
		ToolAnnotations: commandmeta.ToolAnnotations{OpenWorldHint: true},
		DataAccess:      []commandmeta.DataAccess{commandmeta.DataPublic},
	})
}

func setGerritPushURL(cmd *cobra.Command, directory, remote, project, protocol, username string) error {
	address := gerritPushURL(project, protocol, username)
	if err := runGerritGit(cmd, directory, "remote", "set-url", "--push", "--", remote, address); err != nil {
		return fmt.Errorf("set push URL for %s: %w", remote, err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Git remote %s (push): %s\n", remote, gerritSetupURL(address))
	return nil
}

func configureGerritPush(cmd *cobra.Command, remote, protocol string) error {
	if err := validatePushProtocol(protocol); err != nil {
		return err
	}
	ctx := cmd.Context()
	pushURL, err := gitOutput(ctx, "", "remote", "get-url", "--push", "--all", "--", remote)
	if err != nil || pushURL == "" {
		return fmt.Errorf("could not read push URL for Git remote %q", remote)
	}
	if strings.Contains(pushURL, "\n") {
		return fmt.Errorf("Git remote %q has multiple push URLs; choose a remote with one destination", remote)
	}
	project := gerritRemoteProject(gerritSetupURL(pushURL))
	if project == "" {
		fetchURL, _ := gitOutput(ctx, "", "config", "--get", "remote."+remote+".url")
		project = gerritRemoteProject(gerritSetupURL(fetchURL))
	}
	if project == "" {
		return fmt.Errorf("Git remote %q does not identify a Wikimedia Gerrit project", remote)
	}
	username, _, err := gerritIdentity(cmd)
	if err != nil {
		return err
	}
	if username == "" && protocol == "ssh" {
		if previous := gerritSetupURL(pushURL); previous != nil && previous.Scheme == "ssh" {
			username = previous.User.Username()
		}
	}
	return setGerritPushURL(cmd, "", remote, project, protocol, username)
}
