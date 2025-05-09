package codesearch

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/codesearch"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

func NewCodeSearchSearchCmd() *cobra.Command {
	out := output.Output{
		TableBinding: &output.TableBinding{
			Headings: []string{"Repository", "File", "Line", "Match"},
			ProcessObjects: func(objects map[interface{}]interface{}, table *output.Table) {
				typedObject := make(map[string]codesearch.ResultObject, len(objects))
				err := mapstructure.Decode(objects, &typedObject)
				if err != nil {
					panic(err)
				}
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
			err := mapstructure.Decode(objects, &typedObject)
			if err != nil {
				panic(err)
			}
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
		Aliases: []string{"s"},
		Example: cobrautil.NormalizeExample(`
search addshore
search --type extensions --repos "Extension:Wikibase" addshore
search --files ".*\.md" addshore
search --output ack --files ".*\.md" addshore
`),
		Short: "Search using codesearch",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			searchString := args[0]

			searchOptions := &codesearch.SearchOptions{
				IgnoreCase:   ignoreCase,
				Files:        files,
				ExcludeFiles: excludeFiles,
				Repos:        repos,
			}

			if output.Type(out.Type) == output.WebType {
				url := codesearch.CraftSearchURL(searchType, false, searchString, searchOptions)

				fmt.Println("Opening", url)
				browser.OpenURL(url)
				return nil
			}

			client := codesearch.NewClient(searchType)
			response, err := client.Search(context.Background(), searchType, searchString, searchOptions)
			if err != nil {
				return err
			}

			objects := make(map[interface{}]interface{}, len(response.Results))
			for key, result := range response.Results {
				objects[key] = result
			}

			out.Print(cmd, objects)
			return nil
		},
	}
	out.AddFlags(cmd, output.TableType, output.WebType)
	// TODO automate generation of this list?
	// Type can currently be updated from https://github.com/wikimedia/labs-codesearch/blob/master/manage.sh
	// This would probably be a "tool" that is periodically run, to update a set of strings to be included here. (as there is no API spec for this)
	cmd.Flags().StringVarP(&searchType, "type", "t", "search", "Type of search to perform: search|core|extensions|skins|things|bundled|deployed|libraries|operations|puppet|ooui|milkshake|pywikibot|services|analytics|devtools|wmcs|armchairgm|shouthow")
	cmd.Flags().BoolVarP(&ignoreCase, "ignore-case", "i", false, "Ignore case in search")
	cmd.Flags().StringVar(&files, "files", "", "Search only in files matching this pattern")
	cmd.Flags().StringVar(&excludeFiles, "exclude-files", "", "Exclude files matching this pattern")
	cmd.Flags().StringSliceVar(&repos, "repos", []string{}, "Search only in these repositories")
	return cmd
}
