package tools

import (
	"context"
	"errors"
	"fmt"
	osexec "os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	toolforge "gitlab.wikimedia.org/repos/cloud/toolforge/toolforge-gen-cli/pkg/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

const (
	toolforgeQuickstart = "https://wikitech.wikimedia.org/wiki/Help:Toolforge/Quickstart"
	toolsAdminURL       = "https://toolsadmin.wikimedia.org/"
)

var toolforgeAccountName = regexp.MustCompile(`^[a-zA-Z0-9_][a-zA-Z0-9_.-]*$`)

func newToolforgeSetupCmd(probe func(context.Context, toolforge.Connection, string) (string, error)) *cobra.Command {
	var check bool
	var username string
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Check Toolforge SSH and tool-account access",
		Long: "Check production Toolforge SSH using your existing OpenSSH configuration and keys. " +
			"With --tool, also verify that you can become the selected tool account. " +
			"Check membership-request status in Toolsadmin. The command reports access problems and links to next steps.",
		Example: "setup --check\nsetup --check --tool mytool\nsetup --check --username my-shell-name",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !check {
				return cmd.Help()
			}
			for _, name := range []string{"beta", "lima"} {
				if cmd.Flags().Lookup(name) != nil {
					if enabled, _ := cmd.Flags().GetBool(name); enabled {
						return fmt.Errorf("setup checks production Toolforge; omit --beta and --lima")
					}
				}
			}
			tool, _ := cmd.Flags().GetString("tool")
			if tool != "" && !toolforgeAccountName.MatchString(tool) {
				return fmt.Errorf("provide a Toolforge tool name with --tool, such as mytool")
			}
			if username != "" && !toolforgeAccountName.MatchString(username) {
				return fmt.Errorf("--username must be your Wikimedia developer account's UNIX shell username")
			}

			out := cmd.OutOrStdout()
			login, err := toolforge.NewConnection("", false, false)
			if err != nil {
				return err
			}
			shellUser, err := probe(cmd.Context(), login, username)
			if err != nil {
				if cmd.Context().Err() != nil {
					return cmd.Context().Err()
				}
				output.PrintCheck(out, false, "SSH (%s): %v", login.Host, err)
				fmt.Fprintln(out, "  "+toolforgeSSHNextStep(err))
				return nil
			}
			output.PrintCheck(out, true, "SSH (%s): authenticated as %s", login.Host, shellUser)

			if tool == "" {
				fmt.Fprintf(out, "  Add --tool <name> to check access to a tool account, or use %s toolforge create to create one.\n", cmd.Root().Name())
				return nil
			}

			connection, err := toolforge.NewConnection(tool, false, false)
			if err != nil {
				return err
			}
			toolUser, err := probe(cmd.Context(), connection, shellUser)
			if cmd.Context().Err() != nil {
				return cmd.Context().Err()
			}
			var exit *osexec.ExitError
			if errors.As(err, &exit) && exit.ExitCode() == 255 {
				output.PrintCheck(out, false, "SSH while checking %s: %v", connection.ToolUser, err)
				fmt.Fprintln(out, "  "+toolforgeSSHNextStep(err))
				return nil
			}
			if errors.Is(err, context.DeadlineExceeded) {
				output.PrintCheck(out, false, "Tool account %s: access check timed out; retry the check", connection.ToolUser)
				return nil
			}
			if err != nil || toolUser != connection.ToolUser {
				if err == nil {
					err = fmt.Errorf("expected %s, running as %s", connection.ToolUser, toolUser)
				}
				output.PrintCheck(out, false, "Tool account %s: %v", connection.ToolUser, err)
				fmt.Fprintln(out, "  Confirm the tool name and check your maintainer access in "+toolsAdminURL+"tools/")
				return nil
			}

			output.PrintCheck(out, true, "Tool account %s: accessible", connection.ToolUser)
			fmt.Fprintln(out, "  Build and deploy: "+toolforgeQuickstart+"#Build_and_deploy_your_first_tool")
			return nil
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "Check SSH and access to the selected tool account")
	cmd.Flags().StringVar(&username, "username", "", "Override the SSH username with your UNIX shell username")
	return commandmeta.Set(cmd, commandmeta.Hints{
		ToolAnnotations: commandmeta.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: true},
		DataAccess:      []commandmeta.DataAccess{commandmeta.DataAccountMetadata},
	})
}

func toolforgeSSHNextStep(err error) string {
	message := strings.ToLower(err.Error())
	switch {
	case errors.Is(err, osexec.ErrNotFound):
		return "Install the OpenSSH client and configure your Toolforge key: " + toolforgeQuickstart + "#Connect_to_Toolforge_servers_using_SSH"
	case strings.Contains(message, "host key verification failed"), strings.Contains(message, "remote host identification has changed"):
		return "Verify the Toolforge host fingerprint before accepting the host key: " + toolforgeQuickstart + "#Log_in_to_Toolforge"
	case strings.Contains(message, "permission denied"), strings.Contains(message, "no supported authentication methods"):
		return "Check your Toolforge membership or pending request at " + toolsAdminURL + " and verify your UNIX shell username and SSH key."
	case errors.Is(err, context.DeadlineExceeded), strings.Contains(message, "connect to host"), strings.Contains(message, "resolve hostname"):
		return "Check your network connection to the Toolforge SSH host."
	default:
		return "Follow the SSH setup steps at " + toolforgeQuickstart + "#Connect_to_Toolforge_servers_using_SSH"
	}
}

func probeToolforgeIdentity(ctx context.Context, connection toolforge.Connection, username string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	args := append([]string(nil), connection.SSHOptions...)
	args = append(args, "-T", "-o", "BatchMode=yes", "-o", "ConnectTimeout=5",
		"-o", "StrictHostKeyChecking=yes", "-o", "UpdateHostKeys=no",
		"-o", "ClearAllForwardings=yes", "-o", "PermitLocalCommand=no",
		"-o", "ControlMaster=no", "-o", "ControlPath=none", "-o", "ForwardAgent=no", "-o", "ForwardX11=no",
		"-o", "IgnoreUnknown=ForkAfterAuthentication", "-o", "ForkAfterAuthentication=no")
	if username != "" {
		args = append(args, "-l", username)
	}
	args = append(args, connection.Host, connection.RemoteCommand([]string{"/usr/bin/id", "-un"}))
	command := osexec.CommandContext(ctx, "ssh", args...)
	command.WaitDelay = time.Second
	data, err := command.Output()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		var exit *osexec.ExitError
		if errors.As(err, &exit) {
			if message := strings.TrimSpace(string(exit.Stderr)); message != "" {
				return "", fmt.Errorf("%s: %w", message, err)
			}
		}
		return "", err
	}

	identity := strings.TrimSpace(string(data))
	if lastLine := strings.LastIndexByte(identity, '\n'); lastLine != -1 {
		identity = strings.TrimSpace(identity[lastLine+1:])
	}
	if !toolforgeAccountName.MatchString(identity) {
		return "", fmt.Errorf("Toolforge returned no UNIX account name")
	}
	return identity, nil
}
