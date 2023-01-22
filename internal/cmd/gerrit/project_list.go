package gerrit

import (
	"os"

	"github.com/spf13/cobra"
	cmdutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cmd"
)

func NewGerritProjectListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List Gerrit projects",
		Run: func(cmd *cobra.Command, args []string) {
			ssh := cmdutil.AttachAllIO(sshGerritCommand([]string{"ls-projects"}))
			if err := ssh.Run(); err != nil {
				os.Exit(1)
			}
		},
	}
}
