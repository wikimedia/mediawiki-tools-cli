/*Package cmd is used for command line.

Copyright © 2022 Addshore

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/
package cmd

import (
	"fmt"
	"io/ioutil"
	"os"

	mwclient "cgt.name/pkg/go-mwclient"
	"cgt.name/pkg/go-mwclient/params"
	"github.com/spf13/cobra"
)

var (
	wiki               string
	wikiUser           string
	wikiPassword       string
	wikiPageTitle      string
	wikiPagePutSummary string
	wikiPagePutMinor   bool
)

func NewWikiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wiki",
		Short: "MediaWiki Wiki",
		RunE:  nil,
	}
	cmd.PersistentFlags().StringVar(&wiki, "wiki", "", "URL of wikis api.php")
	cmd.PersistentFlags().StringVar(&wikiUser, "user", "", "A user to interact using")
	cmd.PersistentFlags().StringVar(&wikiPassword, "password", "", "Password of the user to interact with")
	return cmd
}

func NewWikiPageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "page",
		Short: "MediaWiki Wiki Page",
		RunE:  nil,
	}
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

func wikiAttachToCmd(rootCmd *cobra.Command) {
	wikiCmd := NewWikiCmd()
	rootCmd.AddCommand(wikiCmd)
	wikipageCmd := NewWikiPageCmd()
	wikiCmd.AddCommand(wikipageCmd)
	wikipageCmd.AddCommand(NewWikiPagePutCmd())
}
