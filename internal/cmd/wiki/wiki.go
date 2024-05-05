package wiki

import (
	"github.com/sirupsen/logrus"
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
		Short: "Interact with any MediaWiki wiki (WORK IN PROGRESS)",
		RunE:  nil,
	}

	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Service"

	cmd.AddCommand(NewWikiPageCmd())
	cmd.PersistentFlags().StringVar(&wiki, "wiki", "", "URL of wikis api.php")
	cmd.PersistentFlags().StringVar(&wikiUser, "user", "", "A user to interact using")
	cmd.PersistentFlags().StringVar(&wikiPassword, "password", "", "Password of the user to interact with")
	err := cmd.MarkFlagRequired("wiki")
	if err != nil {
		logrus.Error(err)
	}

	return cmd
}
