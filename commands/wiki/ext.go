package wiki

import "github.com/spf13/cobra"

func NewWikiExtCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ext",
		Short: "Use extension-specific wiki commands",
		RunE:  nil,
	}

	cmd.AddCommand(NewWikiExtWikibaseCmd())
	return cmd
}
