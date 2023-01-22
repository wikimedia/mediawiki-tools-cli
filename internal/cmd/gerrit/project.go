package gerrit

import (
	"github.com/spf13/cobra"
)

func NewGerritProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Interact with Gerrit projects",
	}
	cmd.AddCommand(NewGerritProjectListCmd())
	cmd.AddCommand(NewGerritProjectSearchCmd())
	cmd.AddCommand(NewGerritProjectCurrentCmd())
	return cmd
}
