/*Package cmd is used for command line.

Copyright © 2020 Addshore

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
	"context"
	"os"

	"github.com/fatih/color"
	"github.com/rodaine/table"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/codesearch"
)

var codesearchCmd = &cobra.Command{
	Use:   "codesearch",
	Short: "MediaWiki code search",
	RunE:  nil,
}

func NewCodeSearchSearchCmd() *cobra.Command {
	var searchType string
	var ignoreCase bool
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search using codesearch",
		Run: func(cmd *cobra.Command, args []string) {
			searchString := args[0]
			client := codesearch.NewClient(searchType)
			ctx := context.Background()
			response, err := client.Search(ctx, searchType, searchString, &codesearch.SearchOptions{IgnoreCase: ignoreCase})
			if err != nil {
				color.Red("Error: %s", err)
				os.Exit(1)
			}

			headerFmt := color.New(color.FgGreen, color.Underline).SprintfFunc()
			columnFmt := color.New(color.FgYellow).SprintfFunc()

			tbl := table.New("Repository", "File", "Line", "Snippet")
			tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)

			for repository, result := range response.Results {
				for _, fileMatch := range result.Matches {
					for _, lineMatch := range fileMatch.Matches {
						tbl.AddRow(repository, fileMatch.Filename, lineMatch.LineNumber, lineMatch.Line)
					}
				}
			}
			tbl.Print()
		},
	}
	cmd.Flags().StringVarP(&searchType, "type", "t", "search", "Type of search to perform: search|core|extensions|skins|things|bundeled|deployed|libraries|operations|puppet|ooui|milkshake|pywikibot|services|analytics")
	cmd.Flags().BoolVarP(&ignoreCase, "ignore-case", "i", false, "Ignore case in search")
	return cmd
}

func init() {
	rootCmd.AddCommand(codesearchCmd)
	codesearchCmd.AddCommand(NewCodeSearchSearchCmd())
}
