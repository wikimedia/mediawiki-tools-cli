package toolhub

import (
	"github.com/spf13/cobra"
)

func NewToolHubCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "toolhub",
		Short: "Wikimedia Toolhub",
		RunE:  nil,
	}

	toolhubToolsCmd := NewToolhubToolsCmd()
	cmd.AddCommand(toolhubToolsCmd)
	toolhubToolsCmd.AddCommand(NewToolHubToolsListCmd())
	toolhubToolsCmd.AddCommand(NewToolHubToolsSearchCmd())
	toolhubToolsCmd.AddCommand(NewToolhubToolsGetCmd())

	return cmd
}
