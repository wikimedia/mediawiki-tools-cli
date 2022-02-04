package cmd

import (
	"github.com/spf13/cobra"
)

func NewToolHubCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "toolhub",
		Short: "Wikimedia Toolhub",
		RunE:  nil,
	}
}

func toolhubAttachToCmd() *cobra.Command {
	toolHubCmd := NewToolHubCmd()

	toolhubToolsCmd := NewToolhubToolsCmd()
	toolHubCmd.AddCommand(toolhubToolsCmd)
	toolhubToolsCmd.AddCommand(NewToolHubToolsListCmd())
	toolhubToolsCmd.AddCommand(NewToolHubToolsSearchCmd())
	toolhubToolsCmd.AddCommand(NewToolhubToolsGetCmd())

	return toolHubCmd
}
