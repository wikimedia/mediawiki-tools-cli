package wiki

import "github.com/spf13/cobra"

func NewWikiExtWikibaseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wikibase",
		Short: "Manage Wikibase items and properties",
		RunE:  nil,
	}

	cmd.AddCommand(NewWikiExtWikibasePropertyCmd())
	cmd.AddCommand(NewWikiExtWikibaseItemCmd())
	addWikibaseCLICompatibility(cmd)
	return cmd
}

func NewWikiExtWikibasePropertyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "property",
		Short: "Manage Wikibase properties",
		RunE:  nil,
	}

	cmd.AddCommand(NewWikiExtWikibasePropertyPutCmd())
	addWikibaseResources(cmd, "property")
	return cmd
}

func NewWikiExtWikibaseItemCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "item",
		Short: "Manage Wikibase items",
		RunE:  nil,
	}

	cmd.AddCommand(NewWikiExtWikibaseItemPutCmd())
	addWikibaseResources(cmd, "item")
	return cmd
}
