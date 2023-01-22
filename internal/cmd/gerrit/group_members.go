package gerrit

import (
	_ "embed"
	"os"

	"github.com/spf13/cobra"
	cmdutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cmd"
)

//go:embed group_members.example
var groupMembersExample string

func NewGerritGroupMembersCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "members [group name]",
		Short:   "List members of a Gerrit group",
		Args:    cobra.MinimumNArgs(1),
		Example: groupMembersExample,
		Run: func(cmd *cobra.Command, args []string) {
			ssh := cmdutil.AttachAllIO(sshGerritCommand([]string{"ls-members", args[0]}))
			if err := ssh.Run(); err != nil {
				os.Exit(1)
			}
		},
	}
}
