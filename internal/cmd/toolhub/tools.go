package toolhub

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/toolhub"
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
			ProcessObjects: func(objects map[interface{}]interface{}, table *output.Table) {
				for _, object := range objects {
					typedObject := object.(toolhub.Tool)
					table.AddRowS(typedObject.Name, typedObject.Type, typedObject.URL)
				}
			},
		},
		AckBinding: func(objects map[interface{}]interface{}, ack *output.Ack) {
			for _, object := range objects {
				typedObject := object.(toolhub.Tool)
				ack.AddItem(typedObject.Type, typedObject.Name+" ("+typedObject.Type+") @ "+typedObject.URL)
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
		Example: toolhubToolsList,
		Run: func(cmd *cobra.Command, args []string) {
			client := toolhub.NewClient()
			ctx := context.Background()
			tools, err := client.GetTools(ctx, nil)
			if err != nil {
				color.Red("Error: %s", err)
				os.Exit(1)
			}
			out.Print(resultsToObjects(tools.Results, toolType))
		},
	}
	out.AddFlags(cmd, output.TableType)
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
		Example: toolhubToolsSearch,
		Args:    cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			searchString := args[0]
			client := toolhub.NewClient()
			ctx := context.Background()
			tools, err := client.SearchTools(ctx, searchString, nil)
			if err != nil {
				color.Red("Error: %s", err)
				os.Exit(1)
			}
			out.Print(resultsToObjects(tools.Results, toolType))
		},
	}
	out.AddFlags(cmd, output.TableType)
	cmd.Flags().StringVarP(&toolType, "type", "t", "*", "Type of tool: web app┃desktop app┃bot┃gadget┃user script┃command line tool┃coding framework┃other|\"\"")
	return cmd
}

func NewToolhubToolsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "Get a specific Toolhub Tool",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]

			client := toolhub.NewClient()
			ctx := context.Background()
			tool, err := client.GetTool(ctx, name, nil)
			if err != nil {
				color.Red("Error: %s", err)
				os.Exit(1)
			}

			empJSON, _ := json.MarshalIndent(tool, "", "  ")

			fmt.Println(string(empJSON))
		},
	}
}
