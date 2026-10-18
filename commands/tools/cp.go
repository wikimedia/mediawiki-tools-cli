package tools

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	toolforge "gitlab.wikimedia.org/repos/cloud/toolforge/toolforge-gen-cli/pkg/cli"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/lookpath"
)

func newCopyCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "cp [flags] source... destination",
		Short:   "Copy files to a Toolforge account using rsync",
		Args:    cobra.MinimumNArgs(2),
		Example: cobrautil.NormalizeExample(cpExample),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := lookpath.NeedExecutables([]string{"rsync", "ssh"}); err != nil {
				return err
			}
			tool, err := cmd.Flags().GetString("tool")
			if err != nil {
				return err
			}
			beta, err := cmd.Flags().GetBool("beta")
			if err != nil {
				return err
			}
			lima, err := cmd.Flags().GetBool("lima")
			if err != nil {
				return err
			}
			connection, err := toolforge.NewConnection(tool, beta, lima)
			if err != nil {
				return err
			}
			rsyncArgs := []string{"-rtlv", "--rsync-path", connection.RemoteCommand([]string{"rsync"})}
			transport := append([]string{"ssh"}, connection.SSHOptions...)
			for i := range transport {
				transport[i] = toolforge.QuoteShellArgument(transport[i])
			}
			rsyncArgs = append(rsyncArgs, "-e", strings.Join(transport, " "))
			destination := args[len(args)-1]
			if destination == "~" {
				destination = "."
			} else {
				destination = strings.TrimPrefix(destination, "~/")
			}
			protected, err := rsyncProtectsArguments(cmd)
			if err != nil {
				return preserveExitCode(cmd, err)
			}
			if protected {
				rsyncArgs = append(rsyncArgs, "--protect-args")
			} else {
				// Older clients (including macOS's bundled rsync) pass paths
				// through the remote shell without argument protection.
				destination = toolforge.QuoteShellArgument(destination)
			}
			rsyncArgs = append(rsyncArgs, "--")
			rsyncArgs = append(rsyncArgs, args[:len(args)-1]...)
			rsyncArgs = append(rsyncArgs, connection.Host+":"+destination)
			rsync := exec.CommandContext(cmd.Context(), "rsync", rsyncArgs...)
			rsync.Stdin = cmd.InOrStdin()
			rsync.Stdout = cmd.OutOrStdout()
			rsync.Stderr = cmd.ErrOrStderr()
			return preserveExitCode(cmd, rsync.Run())
		},
	}
}

func rsyncProtectsArguments(cmd *cobra.Command) (bool, error) {
	output, err := exec.CommandContext(cmd.Context(), "rsync", "--version").Output()
	if err != nil {
		return false, fmt.Errorf("read rsync version: %w", err)
	}
	// Standard rsync 3.x supports --protect-args. Legacy implementations keep
	// the explicit remote quoting used above instead.
	fields := strings.Fields(strings.SplitN(string(output), "\n", 2)[0])
	if len(fields) >= 3 && fields[0] == "rsync" && fields[1] == "version" {
		major, err := strconv.Atoi(strings.SplitN(fields[2], ".", 2)[0])
		return err == nil && major >= 3, nil
	}
	return false, nil
}
