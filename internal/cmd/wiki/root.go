package wiki

import (
	"github.com/spf13/cobra"
)

var (
	wiki         string
	wikiUser     string
	wikiPassword string
)

func NewWikiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wiki",
		Short: "MediaWiki Wiki",
		RunE:  nil,
	}
	cmd.AddCommand(NewWikiPageCmd())
	cmd.PersistentFlags().StringVar(&wiki, "wiki", "", "URL of wikis api.php")
	cmd.PersistentFlags().StringVar(&wikiUser, "user", "", "A user to interact using")
	cmd.PersistentFlags().StringVar(&wikiPassword, "password", "", "Password of the user to interact with")
	return cmd
}
