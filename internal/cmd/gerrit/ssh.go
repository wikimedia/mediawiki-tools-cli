package gerrit

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	cmdutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cmd"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/lookpath"
)

func NewGerritSSHCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh",
		Short: "Gerrits SSH interface",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			cmd.Root().PersistentPreRun(cmd, args)
			if _, err := lookpath.NeedExecutables([]string{"ssh"}); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		},
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
