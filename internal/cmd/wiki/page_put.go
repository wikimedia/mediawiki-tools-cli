package wiki

import (
	_ "embed"
	"fmt"
	"io/ioutil"
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
		wikiPagePutSummary string
		wikiPagePutMinor   bool
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

			bytes, err := ioutil.ReadAll(os.Stdin)
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
				panic(err)
			}

			// https://www.mediawiki.org/wiki/API:Edit#Parameters
			editParams := params.Values{
				"title":   wikiPageTitle,
				"text":    text,
				"summary": wikiPagePutSummary,
			}
			if wikiPagePutMinor {
				editParams["minor"] = "1"
			}

			editErr := w.Edit(editParams)
			if editErr != nil {
				fmt.Println(editErr)
			}
		},
	}

	cmd.Flags().StringVar(&wikiPagePutSummary, "summary", "mwcli edit", "Summary of the edit")
	cmd.Flags().BoolVar(&wikiPagePutMinor, "minor", false, "Minor edit")

	return cmd
}
