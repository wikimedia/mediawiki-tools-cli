package gerrit

import (
	"github.com/spf13/cobra"
)

func NewGerritAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate mw with Wikimedia Gerrit",
	}
	cmd.AddCommand(NewGerritAuthLoginCmd())
	cmd.AddCommand(NewGerritAuthLogoutCmd())
	cmd.AddCommand(NewGerritAuthStatusCmd())
	return cmd
}
