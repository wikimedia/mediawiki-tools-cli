package gerrit

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	osexec "os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	gerritapi "gitlab.wikimedia.org/repos/releng/cli/internal/gerrit"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
	"golang.org/x/crypto/ssh"
)

const (
	gerritSSHHost = "gerrit.wikimedia.org"
	gerritSSHPort = "29418"
)

func newSetupCmd() *cobra.Command {
	var check, installHook bool
	var protocol, remote string
	cmd := &cobra.Command{
		Use:     "setup",
		Short:   "Diagnose your Gerrit account and Git checkout",
		Long:    "Check HTTP and OpenSSH authentication separately. In a Git checkout, show Gerrit remotes, the Git author email, and the active commit-msg hook. Use --check for a read-only report, --install-hook to install the default hook, or --push-protocol to configure a push URL in this checkout.",
		Example: "setup --check\nsetup --install-hook\nsetup --push-protocol https --remote origin",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if cmd.Flags().Changed("push-protocol") {
				if err := configureGerritPush(cmd, remote, protocol); err != nil {
					return err
				}
			}
			if installHook {
				if err := gerritapi.InstallCommitHook(cmd.Context(), ".", gerritapi.CommitHookURL); err != nil {
					return err
				}
				if !checkGerritHook(cmd) {
					return fmt.Errorf("no usable commit-msg hook is configured")
				}
			}
			if !check {
				if installHook || cmd.Flags().Changed("push-protocol") {
					return nil
				}
				return cmd.Help()
			}
			username, password, err := resolveGerritCredentials(cmd)
			sshTargets := checkGerritCheckout(cmd)
			if len(sshTargets) == 0 {
				sshTargets = []gerritSSHTarget{{user: username, port: gerritSSHPort}}
			}
			sshNeedsKey := false
			for _, target := range sshTargets {
				if checkGerritSSH(cmd, target) {
					sshNeedsKey = true
				}
			}
			if err == nil {
				username, err = checkGerritHTTP(cmd.Context(), username, password, cmd.Root().Name())
			}
			if err != nil {
				output.PrintCheck(cmd.OutOrStdout(), false, "HTTP credentials: %v", err)
			} else {
				output.PrintCheck(cmd.OutOrStdout(), true, "HTTP credentials: authenticated as %s", username)
				if sshNeedsKey {
					showRegisteredKeys(cmd, username, password)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "Check the current setup without making changes")
	cmd.Flags().BoolVar(&installHook, "install-hook", false, "Install the commit-msg hook in the current checkout")
	cmd.Flags().StringVar(&protocol, "push-protocol", "", "Set the push URL to SSH or HTTPS in this checkout")
	cmd.Flags().StringVar(&remote, "remote", "origin", "Git remote to configure with --push-protocol")
	cmd.MarkFlagsMutuallyExclusive("check", "install-hook")
	cmd.MarkFlagsMutuallyExclusive("check", "push-protocol")
	return commandmeta.Set(cmd, commandmeta.Hints{
		ToolAnnotations: commandmeta.ToolAnnotations{IdempotentHint: true, OpenWorldHint: true},
		DataAccess:      []commandmeta.DataAccess{commandmeta.DataPrivateMetadata, commandmeta.DataAccountMetadata, commandmeta.DataSecrets},
	})
}

type gerritSSHTarget struct {
	user string
	port string
}

func checkGerritCheckout(cmd *cobra.Command) []gerritSSHTarget {
	git := func(args ...string) (string, error) {
		output, err := osexec.CommandContext(cmd.Context(), "git", args...).Output()
		return strings.TrimSpace(string(output)), err
	}
	if _, err := osexec.LookPath("git"); err != nil {
		output.PrintCheck(cmd.OutOrStdout(), false, "Git checkout: Git is unavailable")
		return nil
	}
	if inside, _ := git("rev-parse", "--is-inside-work-tree"); inside != "true" {
		output.PrintCheck(cmd.OutOrStdout(), false, "Git checkout: run inside a Git worktree to check its remotes and commit-msg hook")
		return nil
	}

	var targets []gerritSSHTarget
	seen := make(map[gerritSSHTarget]bool)
	found := false
	if remotes, err := git("remote"); err != nil {
		output.PrintCheck(cmd.OutOrStdout(), false, "Git remotes: could not list remotes (%v)", err)
	} else {
		for _, name := range strings.Fields(remotes) {
			for _, kind := range []string{"fetch", "push"} {
				args := []string{"remote", "get-url", "--all"}
				if kind == "push" {
					args = append(args, "--push")
				}
				urls, err := git(append(args, "--", name)...)
				if err != nil {
					output.PrintCheck(cmd.OutOrStdout(), false, "Git remote %s (%s): could not resolve URL (%v)", name, kind, err)
					continue
				}
				for address := range strings.SplitSeq(urls, "\n") {
					remote := gerritSetupURL(address)
					if remote == nil {
						continue
					}
					found = true
					output.PrintCheck(cmd.OutOrStdout(), true, "Git remote %s (%s): %s", name, kind, remote)
					if kind != "push" || remote.Scheme != "ssh" {
						continue
					}
					target := gerritSSHTarget{user: remote.User.Username(), port: remote.Port()}
					if !seen[target] {
						targets = append(targets, target)
						seen[target] = true
					}
				}
			}
		}
		if !found {
			output.PrintCheck(cmd.OutOrStdout(), false, "Git remotes: no effective URL points directly to gerrit.wikimedia.org")
		}
	}
	if author, err := git("var", "GIT_AUTHOR_IDENT"); err == nil {
		if i, j := strings.LastIndex(author, "<"), strings.LastIndex(author, ">"); i >= 0 && j > i {
			output.PrintCheck(cmd.OutOrStdout(), true, "Git author email: %s", author[i+1:j])
		}
	} else {
		output.PrintCheck(cmd.OutOrStdout(), false, "Git author email: check git config user.email")
	}
	checkGerritHook(cmd)
	if sshCommand, _ := git("config", "--get", "core.sshCommand"); sshCommand != "" ||
		os.Getenv("GIT_SSH_COMMAND") != "" || os.Getenv("GIT_SSH") != "" {
		output.PrintCheck(cmd.OutOrStdout(), false, "Git uses a custom SSH command. The authentication check below uses OpenSSH directly.")
	}
	return targets
}

func checkGerritHook(cmd *cobra.Command) bool {
	if hooksPath, err := gitOutput(cmd.Context(), "", "config", "--get", "core.hooksPath"); err == nil && hooksPath == "" {
		output.PrintCheck(cmd.OutOrStdout(), false, "commit-msg hook: disabled by empty core.hooksPath")
		return false
	}
	hook, err := gitOutput(cmd.Context(), "", "rev-parse", "--git-path", "hooks/commit-msg")
	if err != nil {
		output.PrintCheck(cmd.OutOrStdout(), false, "commit-msg hook: could not resolve path (%v)", err)
		return false
	}
	switch info, err := os.Stat(hook); {
	case os.IsNotExist(err):
		output.PrintCheck(cmd.OutOrStdout(), false, "commit-msg hook: missing at %s", hook)
		if _, err := gitOutput(cmd.Context(), "", "config", "--get", "core.hooksPath"); err == nil {
			fmt.Fprintf(cmd.OutOrStdout(), "  Configure the hook at your managed hooks path: %s\n", gerritapi.CommitHookURL)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "  Install it with %s gerrit setup --install-hook\n", cmd.Root().Name())
		}
	case err != nil:
		output.PrintCheck(cmd.OutOrStdout(), false, "commit-msg hook: %v", err)
	case !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0):
		output.PrintCheck(cmd.OutOrStdout(), false, "commit-msg hook: %s is not executable", hook)
	default:
		output.PrintCheck(cmd.OutOrStdout(), true, "commit-msg hook: found at %s", hook)
		return true
	}
	return false
}

// Only display known Gerrit URLs, without embedded passwords or query parameters.
func gerritSetupURL(address string) *url.URL {
	address = strings.TrimPrefix(address, "https::")
	if !strings.Contains(address, "://") {
		if host, path, ok := strings.Cut(address, ":"); ok {
			address = "ssh://" + host + "/" + strings.TrimLeft(path, "/")
		}
	}
	remote, err := url.Parse(address)
	if err != nil || !strings.EqualFold(remote.Hostname(), gerritSSHHost) {
		return nil
	}
	switch remote.Scheme {
	case "ssh":
		if remote.User != nil {
			remote.User = url.User(remote.User.Username())
		}
	case "http", "https":
		remote.User = nil
	default:
		return nil
	}
	remote.RawQuery, remote.Fragment, remote.RawFragment, remote.ForceQuery = "", "", "", false
	return remote
}

func checkGerritSSH(cmd *cobra.Command, target gerritSSHTarget) bool {
	// Run a non-mutating SSH command with existing host trust, without prompts,
	// forwarding, host key updates, or reuse of an existing control connection.
	args := []string{
		"-T", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=yes",
		"-o", "UpdateHostKeys=no", "-o", "ControlPath=none", "-o", "ClearAllForwardings=yes",
		"-o", "ForwardAgent=no", "-o", "ForwardX11=no", "-o", "PermitLocalCommand=no", "-o", "ConnectTimeout=5",
	}
	if target.port != "" {
		args = append(args, "-p", target.port)
	}
	if target.user != "" {
		args = append(args, "-l", target.user)
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), gerritRequestTimeout)
	defer cancel()
	if config, err := osexec.CommandContext(ctx, "ssh", append([]string{"-G"}, append(args, gerritSSHHost)...)...).Output(); err == nil {
		for line := range strings.SplitSeq(string(config), "\n") {
			name, value, _ := strings.Cut(line, " ")
			switch name {
			case "user":
				target.user = value
			case "port":
				target.port = value
			}
		}
	}

	address := gerritSSHHost
	if target.user != "" {
		address = target.user + "@" + address
	}
	if target.port != "" {
		address += ":" + target.port
	}
	probe := osexec.CommandContext(ctx, "ssh", append(args, gerritSSHHost, "gerrit", "version")...)
	_, err := probe.Output()
	if err == nil {
		output.PrintCheck(cmd.OutOrStdout(), true, "SSH (%s): authenticated", address)
		return false
	}
	if ctx.Err() != nil {
		err = ctx.Err()
	} else if failure, ok := err.(*osexec.ExitError); ok && len(failure.Stderr) != 0 {
		err = fmt.Errorf("%s", strings.TrimSpace(string(failure.Stderr)))
	}
	output.PrintCheck(cmd.OutOrStdout(), false, "SSH (%s): %v", address, err)
	message := strings.ToLower(err.Error())
	out := cmd.OutOrStdout()
	switch {
	case strings.Contains(message, "remote host identification has changed"), strings.Contains(message, "host key verification failed"):
		fmt.Fprintln(out, "  Verify the Gerrit host key: https://www.mediawiki.org/wiki/Gerrit/Tutorial#Set_Up_SSH_Keys_in_Gerrit")
	case strings.Contains(message, "permission denied") || strings.Contains(message, "no supported authentication methods"):
		fmt.Fprintf(out, "  Check ssh-add -l and the SSH username above. Register a public key with %s gerrit ssh-key add <public-key-file>.\n", cmd.Root().Name())
		return true
	case ctx.Err() != nil || strings.Contains(message, "connect to host") || strings.Contains(message, "resolve hostname"):
		fmt.Fprintln(out, "  Check network access to the SSH host and port above. Gerrit also supports Git over HTTPS.")
	}
	return false
}

func showRegisteredKeys(cmd *cobra.Command, username, password string) {
	ctx, cancel := context.WithTimeout(cmd.Context(), gerritRequestTimeout)
	defer cancel()
	client, err := newGerritClient(ctx)
	if err != nil {
		return
	}
	client.Authentication.SetBasicAuth(username, password)
	keys, response, err := client.Accounts.ListSSHKeys(ctx, "self")
	if err != nil {
		if response != nil {
			response.Body.Close()
		}
		output.PrintCheck(cmd.OutOrStdout(), false, "Registered SSH keys: %v", err)
		return
	}
	out := cmd.OutOrStdout()
	if len(*keys) == 0 {
		fmt.Fprintf(out, "  No public keys registered for %s.\n", username)
		return
	}
	fmt.Fprintf(out, "  Registered keys for %s:\n", username)
	for _, entry := range *keys {
		data, err := base64.StdEncoding.DecodeString(entry.EncodedKey)
		if err == nil {
			if key, err := ssh.ParsePublicKey(data); err == nil {
				status := ""
				if !entry.Valid {
					status = " (invalid)"
				}
				fmt.Fprintf(out, "    %s %s%s\n", ssh.FingerprintSHA256(key), key.Type(), status)
				continue
			}
		}
		fmt.Fprintf(out, "    Key %d (%s) could not be fingerprinted\n", entry.Seq, entry.Algorithm)
	}
}
