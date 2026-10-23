package cloudvps

import (
	"bytes"
	"context"
	"fmt"
	"os"
	osexec "os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servers"
	"github.com/spf13/cobra"
)

func NewComputeSSHCmd() *cobra.Command {
	return newComputeSSHCmd(authenticatedProvider)
}

func newComputeSSHCmd(provider providerFactory) *cobra.Command {
	var user, jump, scriptPath string
	var tty, sudo bool
	cmd := &cobra.Command{
		Use:     "ssh <instance-id> [flags] -- [command [args...]]",
		Short:   "Open an SSH session or execute a command on a Cloud VPS instance",
		Long:    "Connect using the instance's eqiad1 Cloud VPS DNS name and your local SSH configuration. Use --jump for a bastion if it is not already configured in ~/.ssh/config. Remote command arguments are passed literally; use sh -c explicitly for shell expressions. With --script, upload a local Bash script to a private temporary file, run it with the arguments after --, and remove it afterwards. Use --tty for interactive prompts. --sudo runs the script through passwordless sudo.",
		Args:    cobra.MinimumNArgs(1),
		Example: "ssh <instance-id> --project mwcli --user <shell-username> --jump primary.bastion.wmcloud.org\nssh <instance-id> --project mwcli -- uname -a\nssh <instance-id> --project mwcli --script tools/ci/runner.sh --sudo --tty -- setup",
		RunE: func(cmd *cobra.Command, args []string) error {
			if sudo && scriptPath == "" {
				return fmt.Errorf("--sudo requires --script")
			}
			var script []byte
			if scriptPath != "" {
				var err error
				script, err = os.ReadFile(scriptPath)
				if err != nil {
					return fmt.Errorf("read local script: %w", err)
				}
			}
			if _, err := osexec.LookPath("ssh"); err != nil {
				return fmt.Errorf("SSH requires the local ssh executable: %w", err)
			}
			project, err := projectName(cmd)
			if err != nil {
				return err
			}
			p, err := provider(cmd)
			if err != nil {
				return err
			}
			client, err := openstack.NewComputeV2(p, gophercloud.EndpointOpts{})
			if err != nil {
				return err
			}
			server, err := servers.Get(cmd.Context(), client, args[0]).Extract()
			if err != nil {
				return err
			}
			if scriptPath != "" {
				return runSSHScript(cmd, server.Name, project, user, jump, tty, sudo, script, args[1:])
			}
			sshArgs, err := instanceSSHArgs(server.Name, project, user, jump, tty, args[1:])
			if err != nil {
				return err
			}
			ssh := osexec.CommandContext(cmd.Context(), "ssh", sshArgs...)
			ssh.Stdin, ssh.Stdout, ssh.Stderr = cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr()
			return ssh.Run()
		},
	}
	addProjectFlag(cmd)
	cmd.Flags().StringVar(&user, "user", "", "SSH username (defaults to local SSH configuration)")
	cmd.Flags().StringVar(&jump, "jump", "", "SSH jump host, e.g. primary.bastion.wmcloud.org")
	cmd.Flags().BoolVarP(&tty, "tty", "t", false, "Allocate a terminal for a remote command (automatic for interactive sessions)")
	cmd.Flags().StringVar(&scriptPath, "script", "", "Upload and execute a local Bash script; arguments after -- are passed to it")
	cmd.Flags().BoolVar(&sudo, "sudo", false, "Run --script as root using sudo -n (requires passwordless sudo)")
	return cmd
}

var remoteScriptPath = regexp.MustCompile(`^/tmp/mwcli-script\.[A-Za-z0-9]+$`)

func runSSHScript(cmd *cobra.Command, name, project, user, jump string, tty, sudo bool, script []byte, scriptArgs []string) error {
	// Upload without a PTY so terminal processing cannot alter the script.
	upload := `set -eu
script=$(mktemp /tmp/mwcli-script.XXXXXXXX)
trap 'rm -f -- "$script"' EXIT
trap 'exit 1' HUP INT TERM
cat > "$script"
printf '%s\n' "$script"
trap - EXIT`
	args, err := instanceSSHArgs(name, project, user, jump, false, []string{"sh", "-c", upload})
	if err != nil {
		return err
	}
	args = append([]string{"-T"}, args...)
	var uploaded bytes.Buffer
	ssh := osexec.CommandContext(cmd.Context(), "ssh", args...)
	ssh.Stdin, ssh.Stdout, ssh.Stderr = bytes.NewReader(script), &uploaded, cmd.ErrOrStderr()
	if err := ssh.Run(); err != nil {
		return fmt.Errorf("upload script: %w", err)
	}
	path := strings.TrimSpace(uploaded.String())
	if !remoteScriptPath.MatchString(path) {
		return fmt.Errorf("upload returned an unexpected temporary path")
	}
	// Also attempt cleanup if the execution connection fails or is cancelled.
	defer func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(cmd.Context()), 10*time.Second)
		defer cancel()
		cleanupArgs, _ := instanceSSHArgs(name, project, user, jump, false, []string{"rm", "-f", "--", path})
		cleanupArgs = append([]string{"-T"}, cleanupArgs...)
		if err := osexec.CommandContext(ctx, "ssh", cleanupArgs...).Run(); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Could not remove remote temporary script %s: %v\n", path, err)
		}
	}()
	run := []string{"sh", "-c", `script=$1; shift; trap 'rm -f -- "$script"' EXIT; "$@"`, "sh", path}
	if sudo {
		run = append(run, "sudo", "-n", "--")
	}
	run = append(run, "bash", path)
	run = append(run, scriptArgs...)
	args, err = instanceSSHArgs(name, project, user, jump, tty, run)
	if err != nil {
		return err
	}
	ssh = osexec.CommandContext(cmd.Context(), "ssh", args...)
	ssh.Stdin, ssh.Stdout, ssh.Stderr = cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr()
	return ssh.Run()
}

func instanceSSHArgs(name, project, user, jump string, tty bool, command []string) ([]string, error) {
	if !dnsLabel.MatchString(name) || !dnsLabel.MatchString(project) {
		return nil, fmt.Errorf("instance name and project must be lowercase DNS labels to construct the SSH hostname")
	}
	args := []string{}
	if user != "" {
		args = append(args, "-l", user)
	}
	if jump != "" {
		args = append(args, "-J", jump)
	}
	if tty || len(command) == 0 {
		args = append(args, "-t")
	}
	args = append(args, name+"."+project+".eqiad1.wikimedia.cloud")
	if len(command) > 0 {
		quoted := make([]string, len(command))
		for i, arg := range command {
			quoted[i] = "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
		}
		args = append(args, strings.Join(quoted, " "))
	}
	return args, nil
}
