package auth

import (
	"github.com/spf13/cobra"
)

func NewDeveloperCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "developer",
		Short: "Manage Wikimedia developer account authentication",
	}

	cmd.AddCommand(NewDeveloperCreateCmd())
	cmd.AddCommand(NewDeveloperAgentCmd())

	return cmd
}