package gerrit

import (
	"github.com/spf13/cobra"
	cmdutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cmd"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/lookpath"
)

func NewGerritSSHCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh",
		Short: "Gerrit's SSH interface",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := lookpath.NeedExecutables([]string{"ssh"}); err != nil {
				return err
			}
			ssh := cmdutil.AttachInErrIO(sshGerritCommand(args))
			out := cmdutil.AttachOutputBuffer(ssh)

			err := ssh.Run()
			cmd.Printf("%s", out)
			return err
		},
	}
	return cmd
}
