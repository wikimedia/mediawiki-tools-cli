package toolhub

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/toolhub"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

// TODO split file into separate files per command

func NewToolhubToolsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "Toolhub tools",
	}
	cmd.AddCommand(NewToolHubToolsListCmd())
	cmd.AddCommand(NewToolHubToolsSearchCmd())
	cmd.AddCommand(NewToolhubToolsGetCmd())
	return cmd
}

func toolOutput() output.Output {
	return output.Output{
		TableBinding: &output.TableBinding{
			Headings: []string{"Name", "Type", "URL"},
			RowExtractor: func(object interface{}) []string {
				typedObject, ok := object.(toolhub.Tool)
				if !ok {
					return nil
				}
				return []string{typedObject.Name, typedObject.Type, typedObject.URL}
			},
		},
		PrettyBinding: func(objects interface{}, pretty *output.Pretty) {
			objMap, ok := objects.(map[interface{}]interface{})
			if ok {
				for _, object := range objMap {
					typedObject, ok := object.(toolhub.Tool)
					if !ok {
						continue
					}
					pretty.AddItem(typedObject.Type, typedObject.Name+" @ "+typedObject.URL)
				}
			}
		},
	}
}

func resultsToObjects(results []toolhub.Tool, toolType string) map[interface{}]interface{} {
	objects := make(map[interface{}]interface{}, len(results))
	for key, tool := range results {
		if toolType == "*" || toolType == tool.Type {
			objects[key] = tool
		}
	}
	return objects
}

//go:embed toolhub_tools_list.example
var toolhubToolsList string

func NewToolHubToolsListCmd() *cobra.Command {
	var toolType string

	out := toolOutput()
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List Toolhub Tools",
		Example: cobrautil.NormalizeExample(toolhubToolsList),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := toolhub.NewClient()
			ctx := cmd.Context()
			tools, err := client.GetTools(ctx, nil)
			if err != nil {
				return err
			}
			return out.Print(cmd, resultsToObjects(tools.Results, toolType))
		},
	}
	out.AddFlagsWithOpts(cmd, output.WithDefaultTTY(output.PrettyType), output.WithDefaultPipe(output.JSONType))
	cmd.Flags().StringVarP(&toolType, "type", "t", "*", "Type of tool: web app┃desktop app┃bot┃gadget┃user script┃command line tool┃coding framework┃other|\"\"")
	return cmd
}

//go:embed toolhub_tools_search.example
var toolhubToolsSearch string

func NewToolHubToolsSearchCmd() *cobra.Command {
	var toolType string

	out := toolOutput()
	cmd := &cobra.Command{
		Use:     "search",
		Short:   "Search Toolhub Tools",
		Example: cobrautil.NormalizeExample(toolhubToolsSearch),
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			searchString := args[0]
			client := toolhub.NewClient()
			ctx := cmd.Context()
			tools, err := client.SearchTools(ctx, searchString, nil)
			if err != nil {
				return err
			}
			return out.Print(cmd, resultsToObjects(tools.Results, toolType))
		},
	}
	out.AddFlagsWithOpts(cmd, output.WithDefaultTTY(output.PrettyType), output.WithDefaultPipe(output.JSONType))
	cmd.Flags().StringVarP(&toolType, "type", "t", "*", "Type of tool: web app┃desktop app┃bot┃gadget┃user script┃command line tool┃coding framework┃other|\"\"")
	return cmd
}

func NewToolhubToolsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "Get a specific Toolhub Tool",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			client := toolhub.NewClient()
			ctx := cmd.Context()
			tool, err := client.GetTool(ctx, name, nil)
			if err != nil {
				return err
			}

			data, err := json.MarshalIndent(tool, "", "  ")
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return err
		},
	}
}
