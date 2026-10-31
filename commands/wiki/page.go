package wiki

import (
	"github.com/spf13/cobra"
)

var wikiPageTitle string

func NewWikiPageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "page",
		Short: "Read and manage wiki pages",
		RunE:  nil,
	}

	cmd.AddCommand(NewWikiPagePutCmd())
	cmd.AddCommand(NewWikiPageGetCmd())
	cmd.AddCommand(NewWikiPageDeleteCmd())
	cmd.AddCommand(NewWikiPageListCmd())
	cmd.AddCommand(NewWikiPageMoveCmd())
	cmd.PersistentFlags().StringVar(&wikiPageTitle, "title", "", "Title of the page")
	// Keep accepting --title before a subcommand for existing scripts.
	_ = cmd.PersistentFlags().MarkHidden("title")

	return cmd
}
