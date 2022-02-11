package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	cmdutil "gitlab.wikimedia.org/releng/cli/internal/util/cmd"
)

func NewGerritSSHCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh",
		Short: "Gerrits SSH interface",
		Run: func(cmd *cobra.Command, args []string) {
			ssh := cmdutil.AttachInErrIO(sshGerritCommand(args))
			out := cmdutil.AttachOutputBuffer(ssh)

			err := ssh.Run()
			fmt.Printf("%s", out)
			if err != nil {
				os.Exit(1)
			}
		},
	}
	return cmd
}
