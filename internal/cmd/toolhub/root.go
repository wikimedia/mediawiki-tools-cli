package toolhub

import (
	"github.com/spf13/cobra"
)

func NewToolHubCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "toolhub",
		Short: "Interact with the Wikimedia Toolhub",
		RunE:  nil,
	}

	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Service"

	cmd.AddCommand(NewToolhubToolsCmd())

	return cmd
}
