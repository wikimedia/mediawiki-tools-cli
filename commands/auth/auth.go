package auth

import (
	"github.com/spf13/cobra"
)

func NewAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "auth",
		Short:  "Manage authentication",
		Hidden: true,
	}

	cmd.AddCommand(NewDeveloperCmd())

	return cmd
}
