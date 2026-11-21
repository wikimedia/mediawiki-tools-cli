package gitlab

import (
	"context"
	"fmt"
	"net/url"
	"os"
	osexec "os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gitlab.com/gitlab-org/cli/mwcli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/gitconfig"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

func newGitlabSetupCmd(session *profileSession) *cobra.Command {
	var check bool
	var remote string
	cmd := &cobra.Command{
		Use:     "setup",
		Short:   "Check your GitLab profile and Git repository access",
		Long:    "Verify the saved GitLab profile, show Git’s effective fetch and push URLs, and check whether the repository is readable over the push transport. The check leaves Git configuration untouched and never uploads commits.",
		Example: "setup --check\nsetup --check --remote origin",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !check {
				return cmd.Help()
			}
			out := cmd.OutOrStdout()
			profile, err := session.credentials()
			if err == nil && profile == nil {
				err = fmt.Errorf("no saved GitLab profile; use mw gitlab auth login")
			}
			if err == nil {
				ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
				err = checkGitlabProfile(ctx, profile)
				cancel()
			}
			if err != nil {
				output.PrintCheck(out, false, "GitLab profile: %v", err)
			} else {
				output.PrintCheck(out, true, "GitLab profile: authenticated as %s on %s", profile.Username, profile.Host)
			}
			checkGitlabRemote(cmd, session, profile, remote)
			return cmd.Context().Err()
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "Check the current setup without making changes")
	cmd.Flags().StringVar(&remote, "remote", "", "Git remote to check (default: Git’s push remote)")
	return commandmeta.Set(cmd, commandmeta.Hints{
		ToolAnnotations: commandmeta.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: true},
		DataAccess:      []commandmeta.DataAccess{commandmeta.DataAccountMetadata, commandmeta.DataPrivateMetadata, commandmeta.DataSecrets},
	})
}

func checkGitlabRemote(cmd *cobra.Command, session *profileSession, profile *mwcli.ProfileCredentials, remote string) {
	ctx := cmd.Context()
	out := cmd.OutOrStdout()
	if _, err := gitlabSetupGit(ctx, "rev-parse", "--git-dir"); err != nil {
		output.PrintCheck(out, false, "Git repository: run inside a GitLab checkout to check Git transport")
		return
	}
	if remote == "" {
		branch, _ := gitlabSetupGit(ctx, "symbolic-ref", "--quiet", "--short", "HEAD")
		remote = gitconfig.PushRemote(ctx, branch)
	}
	if urls, err := gitlabSetupGit(ctx, "remote", "get-url", "--all", "--", remote); err == nil {
		for raw := range strings.SplitSeq(urls, "\n") {
			if address := gitlabRemoteURL(raw); address != nil {
				output.PrintCheck(out, true, "Git remote %s (fetch): %s", remote, address)
			}
		}
	}
	urls, err := gitlabSetupGit(ctx, "remote", "get-url", "--all", "--push", "--", remote)
	if err != nil || urls == "" {
		output.PrintCheck(out, false, "Git remote %s: no push URL; choose one with --remote", remote)
		return
	}
	if strings.Contains(urls, "\n") {
		output.PrintCheck(out, false, "Git remote %s: multiple push URLs; choose a remote with one destination", remote)
		return
	}
	address := gitlabRemoteURL(urls)
	if address == nil {
		output.PrintCheck(out, false, "Git remote %s: use a GitLab SSH or HTTPS URL", remote)
		return
	}
	output.PrintCheck(out, true, "Git remote %s (push): %s", remote, address)

	host := mwcli.Hostname(address.Hostname())
	expectedHost := "gitlab.wikimedia.org"
	if profile != nil {
		if expected, err := url.Parse("https://" + profile.Host); err == nil {
			expectedHost = expected.Hostname()
		}
	}
	if !strings.EqualFold(host, expectedHost) {
		output.PrintCheck(out, false, "Git transport: remote host %s differs from GitLab host %s; select a matching --profile or --remote", host, expectedHost)
		return
	}

	// Only pass the sanitized URL for HTTPS. Let existing SSH config determine
	// the user, port, identity and jump host using the original SSH URL.
	if address.Scheme == "https" {
		urls = "https::" + address.String()
		if profile != nil {
			name, err := session.name()
			if err != nil {
				output.PrintCheck(out, false, "Git transport: %v", err)
				return
			}
			restore, err := profileGitEnvironment(name, profile)
			if err != nil {
				output.PrintCheck(out, false, "Git transport: %v", err)
				return
			}
			defer restore()
		}
	}
	if err := checkGitlabGitRead(ctx, address.Scheme, urls); err != nil {
		output.PrintCheck(out, false, "Git %s access: %v", strings.ToUpper(address.Scheme), err)
		if address.Scheme == "ssh" {
			fmt.Fprintln(out, "  Check your SSH key and host trust for "+address.Hostname())
		} else {
			fmt.Fprintln(out, "  Check the selected GitLab profile and repository permissions")
		}
		return
	}
	output.PrintCheck(out, true, "Git %s access: repository is readable", strings.ToUpper(address.Scheme))
}

func gitlabRemoteURL(raw string) *url.URL {
	raw = strings.TrimPrefix(raw, "https::")
	if !strings.Contains(raw, "://") {
		host, path, ok := strings.Cut(raw, ":")
		if !ok || strings.ContainsAny(host, "/\\") {
			return nil
		}
		raw = "ssh://" + host + "/" + strings.TrimLeft(path, "/")
	}
	address, err := url.Parse(raw)
	if err != nil || address.Hostname() == "" || strings.Trim(address.Path, "/") == "" ||
		(address.Scheme != "ssh" && address.Scheme != "https") {
		return nil
	}
	if address.Scheme == "ssh" && address.User != nil {
		address.User = url.User(address.User.Username())
	} else {
		address.User = nil
	}
	address.RawQuery, address.Fragment, address.RawFragment, address.ForceQuery = "", "", "", false
	return address
}

func gitlabSetupGit(ctx context.Context, args ...string) (string, error) {
	data, err := osexec.CommandContext(ctx, "git", args...).Output()
	return strings.TrimSpace(string(data)), err
}

func checkGitlabGitRead(ctx context.Context, protocol, address string) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	effective, err := gitlabSetupGit(ctx, "ls-remote", "--get-url", "--", address)
	if err != nil || effective != address {
		return fmt.Errorf("Git would rewrite the resolved URL again; check your Git URL rewrites")
	}
	git := osexec.CommandContext(ctx, "git", "-c", "core.askPass=", "ls-remote", "--", address)
	git.Env = append(git.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=", "SSH_ASKPASS_REQUIRE=never",
		"GIT_HTTP_USER_AGENT="+cli.HTTPUserAgent(), "GIT_TRACE_REDACT=1")
	if protocol == "ssh" && os.Getenv("GIT_SSH_COMMAND") == "" && os.Getenv("GIT_SSH") == "" {
		custom, _ := gitlabSetupGit(ctx, "config", "--get", "core.sshCommand")
		if custom == "" {
			git.Env = append(git.Env, "GIT_SSH_COMMAND=ssh -o BatchMode=yes -o StrictHostKeyChecking=yes -o UpdateHostKeys=no -o ClearAllForwardings=yes -o PermitLocalCommand=no -o ControlPath=none -o ForwardAgent=no -o ForwardX11=no")
		}
	}
	// Git and custom SSH commands can include URLs or other configuration in
	// diagnostics. Report the failure without echoing potentially secret values.
	git.WaitDelay = time.Second
	err = git.Run()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return fmt.Errorf("could not read remote refs: %w", err)
	}
	return nil
}
