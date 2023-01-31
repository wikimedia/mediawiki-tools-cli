package gerrit

import (
	"github.com/spf13/cobra"
)

func NewGerritChangesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "changes",
		Short: "Interact with Gerrit changes",
	}
	cmd.AddCommand(NewGerritChangesListCmd())
	return cmd
}
