package wiki

import (
	"github.com/spf13/cobra"
)

var wikiPageTitle string

func NewWikiPageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "page",
		Short: "MediaWiki Wiki Page",
		RunE:  nil,
	}

	cmd.AddCommand(NewWikiPagePutCmd())
	cmd.AddCommand(NewWikiPageDeleteCmd())
	cmd.AddCommand(NewWikiPageListCmd())
	cmd.PersistentFlags().StringVar(&wikiPageTitle, "title", "", "Title of the page")

	return cmd
}
