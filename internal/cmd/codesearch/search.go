package codesearch

import (
	"context"
	_ "embed"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/codesearch"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

//go:embed search.example
var searchExample string

func NewCodeSearchSearchCmd() *cobra.Command {
	out := output.Output{
		TableBinding: &output.TableBinding{
			Headings: []string{"Repository", "File", "Line", "Match"},
			ProcessObjects: func(objects map[interface{}]interface{}, table *output.Table) {
				typedObject := make(map[string]codesearch.ResultObject, len(objects))
				mapstructure.Decode(objects, &typedObject)
				for repository, result := range typedObject {
					for _, fileMatch := range result.Matches {
						for _, lineMatch := range fileMatch.Matches {
							table.AddRow(repository, fileMatch.Filename, lineMatch.LineNumber, lineMatch.Line)
						}
					}
				}
			},
		},
		AckBinding: func(objects map[interface{}]interface{}, ack *output.Ack) {
			typedObject := make(map[string]codesearch.ResultObject, len(objects))
			mapstructure.Decode(objects, &typedObject)
			for repository, result := range typedObject {
				for _, fileMatch := range result.Matches {
					sectionName := repository + " " + fileMatch.Filename
					ack.InitSection(sectionName)
					for _, lineMatch := range fileMatch.Matches {
						ack.AddItem(sectionName, fmt.Sprintf("%d:%s", lineMatch.LineNumber, lineMatch.Line))
					}
				}
			}
		},
	}

	var searchType string
	var files string
	var excludeFiles string
	var repos []string
	var ignoreCase bool
	cmd := &cobra.Command{
		Use:     "search [search-text]",
		Example: searchExample,
		Short:   "Search using codesearch",
		Args:    cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			searchString := args[0]
			client := codesearch.NewClient(searchType)
			ctx := context.Background()
			response, err := client.Search(ctx, searchType, searchString, &codesearch.SearchOptions{
				IgnoreCase:   ignoreCase,
				Files:        files,
				ExcludeFiles: excludeFiles,
				Repos:        repos,
			})
			if err != nil {
				color.Red("Error: %s", err)
				os.Exit(1)
			}

			objects := make(map[interface{}]interface{}, len(response.Results))
			for key, result := range response.Results {
				objects[key] = result
			}

			out.Print(objects)
		},
	}
	out.AddFlags(cmd, string(output.TableType))
	cmd.Flags().StringVarP(&searchType, "type", "t", "search", "Type of search to perform: search|core|extensions|skins|things|bundeled|deployed|libraries|operations|puppet|ooui|milkshake|pywikibot|services|analytics")
	cmd.Flags().BoolVarP(&ignoreCase, "ignore-case", "i", false, "Ignore case in search")
	cmd.Flags().StringVar(&files, "files", "", "Search only in files matching this pattern")
	cmd.Flags().StringVar(&excludeFiles, "exclude-files", "", "Exclude files matching this pattern")
	cmd.Flags().StringSliceVar(&repos, "repos", []string{}, "Search only in these repositories")
	return cmd
}
