package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/codesearch"
	"gitlab.wikimedia.org/releng/cli/internal/util/printers"
)

func NewCodeSearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "codesearch",
		Short: "MediaWiki code search",
		RunE:  nil,
	}
}

func NewCodeSearchSearchCmd() *cobra.Command {
	var output string
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

			if output == "table" {
				table := printers.Table{}
				table.AddHeadings("Repository", "File", "Line", "Match")
				for repository, result := range response.Results {
					for _, fileMatch := range result.Matches {
						for _, lineMatch := range fileMatch.Matches {
							table.AddRow(repository, fileMatch.Filename, lineMatch.LineNumber, lineMatch.Line)
						}
					}
				}
				table.Print()
			}
			if output == "ack" {
				ack := printers.Ack{}
				for repository, result := range response.Results {
					for _, fileMatch := range result.Matches {
						sectionName := repository + " " + fileMatch.Filename
						ack.InitSection(sectionName)
						for _, lineMatch := range fileMatch.Matches {
							ack.AddItem(sectionName, fmt.Sprintf("%d:%s", lineMatch.LineNumber, lineMatch.Line))
						}
					}
				}
				ack.Print()
			}
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "table", "Output types: table|ack")
	cmd.Flags().StringVarP(&searchType, "type", "t", "search", "Type of search to perform: search|core|extensions|skins|things|bundeled|deployed|libraries|operations|puppet|ooui|milkshake|pywikibot|services|analytics")
	cmd.Flags().BoolVarP(&ignoreCase, "ignore-case", "i", false, "Ignore case in search")
	return cmd
}

func codesearchAttachToCmd() *cobra.Command {
	codesearchCmd := NewCodeSearchCmd()
	codesearchCmd.AddCommand(NewCodeSearchSearchCmd())
	return codesearchCmd
}
