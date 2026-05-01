package wiki

import "github.com/spf13/cobra"

func NewWikiExtWikibaseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wikibase",
		Short: "Wikibase extension API operations",
		RunE:  nil,
	}

	cmd.AddCommand(NewWikiExtWikibasePropertyCmd())
	cmd.AddCommand(NewWikiExtWikibaseItemCmd())
	return cmd
}

func NewWikiExtWikibasePropertyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "property",
		Short: "Wikibase property operations",
		RunE:  nil,
	}

	cmd.AddCommand(NewWikiExtWikibasePropertyPutCmd())
	return cmd
}

func NewWikiExtWikibaseItemCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "item",
		Short: "Wikibase item operations",
		RunE:  nil,
	}

	cmd.AddCommand(NewWikiExtWikibaseItemPutCmd())
	return cmd
}
