package toolhub

import (
	"github.com/spf13/cobra"
)

func NewToolHubCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "toolhub",
		GroupID: "service",
		Short:   "Interact with the Wikimedia Toolhub (WORK IN PROGRESS)",
		RunE:    nil,
	}

	cmd.AddCommand(NewToolhubToolsCmd())

	return cmd
}
