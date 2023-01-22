package gerrit

import (
	"github.com/spf13/cobra"
)

func NewGerritGroupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "group",
		Short: "Interact with Gerrit groups",
	}
	cmd.AddCommand(NewGerritGroupListCmd())
	cmd.AddCommand(NewGerritGroupSearchCmd())
	cmd.AddCommand(NewGerritGroupMembersCmd())
	return cmd
}
