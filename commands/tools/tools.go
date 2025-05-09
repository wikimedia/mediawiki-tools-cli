package tools

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	cmdutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cmd"
	sshutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/ssh"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/lookpath"
)

//go:embed exec.example
var execExample string

//go:embed cp.example
var cpExample string

//go:embed exec.long.md
var execLong string

func NewToolsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tools",
		GroupID: "service",
		Aliases: []string{"toolforge"},
		Short:   "Interact with any Wikimedia \"Tool\"",
		RunE:    nil,
	}

	toolName := ""
	// TODO split exec cmd into own function & own file
	execCmd := &cobra.Command{
		Use:     "exec [flags] [command & arguments] -- [command flags]",
		Short:   "Execute commands as a tool",
		Args:    cobra.MinimumNArgs(1),
		Example: execExample,
		Long:    cli.RenderMarkdown(execLong),
		Run: func(cmd *cobra.Command, args []string) {
			if _, err := lookpath.NeedExecutables([]string{"ssh"}); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			remoteCommandAndArgs := args
			if toolName != "" {
				remoteCommandAndArgs = append([]string{"become", toolName}, remoteCommandAndArgs...)
			}

			sshCmd := sshutil.CommandOnSSHHost("login.toolforge.org", "22", remoteCommandAndArgs)
			logrus.Trace(sshCmd.String())

			sshCmd = cmdutil.AttachAllIO(sshCmd)
			err := sshCmd.Run()
			if err != nil {
				logrus.Debugf("ssh command returned an error: %v", err)
				return
			}
		},
	}
	execCmd.Flags().StringVarP(&toolName, "tool", "t", "", "Tool to execute command on")
	cmd.AddCommand(execCmd)

	cpCmd := &cobra.Command{
		Use:     "cp [flags] [source] [destination]",
		Short:   "Copy files to a tool using rsync",
		Args:    cobra.MinimumNArgs(2),
		Example: cpExample,
		Run: func(cmd *cobra.Command, args []string) {
			if _, err := lookpath.NeedExecutables([]string{"rsync"}); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			rsyncPath := "rsync"
			// If a tool is specified, then rsync as the tool
			if toolName != "" {
				rsyncPath = fmt.Sprintf("become %s rsync", toolName)
			}

			source := args[0]
			destination := args[1]

			rsync := exec.Command("rsync", "-rtlv", "--rsync-path", rsyncPath, "--port", "22", source, "login.toolforge.org:"+destination) // #nosec G204
			logrus.Trace(rsync.String())

			rsync = cmdutil.AttachAllIO(rsync)
			err := rsync.Run()
			if err != nil {
				logrus.Debugf("rsync command returned an error: %v", err)
				return
			}
		},
	}
	cpCmd.Flags().StringVarP(&toolName, "tool", "t", "", "Tool to execute command on")
	cmd.AddCommand(cpCmd)

	return cmd
}
