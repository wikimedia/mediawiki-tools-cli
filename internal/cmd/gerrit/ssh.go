package gerrit

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	cmdutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cmd"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/lookpath"
)

func NewGerritSSHCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh",
		Short: "Gerrit's SSH interface",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			cobrautil.CallAllPersistentPreRun(cmd, args)
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
