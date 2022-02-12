package wiki

import (
	"fmt"
	"io/ioutil"
	"os"

	mwclient "cgt.name/pkg/go-mwclient"
	"cgt.name/pkg/go-mwclient/params"
	"github.com/spf13/cobra"
)

var (
	wikiPageTitle      string
	wikiPagePutSummary string
	wikiPagePutMinor   bool
)

func NewWikiPageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "page",
		Short: "MediaWiki Wiki Page",
		RunE:  nil,
	}
	cmd.AddCommand(NewWikiPagePutCmd())
	cmd.PersistentFlags().StringVar(&wikiPageTitle, "title", "", "Title of the page")
	return cmd
}

func NewWikiPagePutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "put",
		Short: "MediaWiki Wiki Page Put",
		RunE:  nil,
		Run: func(cmd *cobra.Command, args []string) {
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
