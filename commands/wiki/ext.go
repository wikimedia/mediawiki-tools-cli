package wiki

import "github.com/spf13/cobra"

func NewWikiExtCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ext",
		Short: "Interact with MediaWiki extensions via API",
		RunE:  nil,
	}

	cmd.AddCommand(NewWikiExtWikibaseCmd())
	return cmd
}
