package wiki

import (
	_ "embed"
	"io"
	"os"

	mwclient "cgt.name/pkg/go-mwclient"
	"cgt.name/pkg/go-mwclient/params"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

//go:embed page_put.example
var pagePutExample string

func NewWikiPagePutCmd() *cobra.Command {
	var (
		summary    string
		minor      bool
		bot        bool
		recreate   bool
		nocreate   bool
		createonly bool
	)

	cmd := &cobra.Command{
		Use:     "put",
		Short:   "MediaWiki Wiki Page Put",
		RunE:    nil,
		Example: pagePutExample,
		Run: func(cmd *cobra.Command, args []string) {
			if wiki == "" {
				logrus.Fatal("wiki is not set")
			}
			if wikiUser == "" {
				logrus.Fatal("wiki is not set")
			}
			if wikiPassword == "" {
				logrus.Fatal("wiki is not set")
			}
			if wikiPageTitle == "" {
				logrus.Fatal("title is not set")
			}

			bytes, err := io.ReadAll(os.Stdin)
			if err != nil {
				panic(err)
			}
			text := string(bytes)

			w, err := mwclient.New(wiki, "mwcli")
			if err != nil {
				panic(err)
			}

			// TODO only login if user and pass is set
			err = w.Login(wikiUser, wikiPassword)
			if err != nil {
				// Print warnings, of fatal on errors
				if _, ok := err.(mwclient.APIWarnings); ok {
					// TODO in the future don't just hide the warnings...
					// // print the warnings
					// for _, warning := range apiWarnings {
					// 	logrus.Warn(warning)
					// }
				} else {
					logrus.Panic(err)
				}
			}

			// https://www.mediawiki.org/wiki/API:Edit#Parameters
			editParams := params.Values{
				"title":   wikiPageTitle,
				"text":    text,
				"summary": summary,
			}
			if minor {
				editParams["minor"] = "1"
			}
			if bot {
				editParams["bot"] = "1"
			}
			if recreate {
				editParams["recreate"] = "1"
			}
			if nocreate {
				editParams["nocreate"] = "1"
			}
			if createonly {
				editParams["createonly"] = "1"
			}

			editErr := w.Edit(editParams)
			if editErr != nil {
				logrus.Panic(editErr)
			}
		},
	}

	cmd.Flags().StringVar(&summary, "summary", "mwcli edit", "Summary of the edit")
	cmd.Flags().BoolVar(&minor, "minor", false, "Minor edit")
	cmd.Flags().BoolVar(&bot, "bot", false, "Bot edit")
	cmd.Flags().BoolVar(&recreate, "recreate", false, "Override any errors about the page having been deleted in the meantime.")
	cmd.Flags().BoolVar(&nocreate, "nocreate", false, "Throw an error if the page doesn't exist.")
	cmd.Flags().BoolVar(&createonly, "createonly", false, "Don't edit the page if it exists already.")

	return cmd
}
