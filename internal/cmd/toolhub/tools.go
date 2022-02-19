package toolhub

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/rodaine/table"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/toolhub"
)

func NewToolhubToolsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "Interact with Toolhub tools",
	}
	cmd.AddCommand(NewToolHubToolsListCmd())
	cmd.AddCommand(NewToolHubToolsSearchCmd())
	cmd.AddCommand(NewToolhubToolsGetCmd())
	return cmd
}

func NewToolHubToolsListCmd() *cobra.Command {
	var toolType string
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List Toolhub Tools",
		Example: "list\nlist --type=\"web app\"\nlist --type=\"\"",
		Run: func(cmd *cobra.Command, args []string) {
			client := toolhub.NewClient()
			ctx := context.Background()
			tools, err := client.GetTools(ctx, nil)
			if err != nil {
				color.Red("Error: %s", err)
				os.Exit(1)
			}

			headerFmt := color.New(color.FgGreen, color.Underline).SprintfFunc()
			columnFmt := color.New(color.FgYellow).SprintfFunc()

			tbl := table.New("Name", "Type", "URL")
			tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)

			for _, tool := range tools.Results {
				if toolType == "*" || toolType == tool.Type || (toolType == "" && tool.Type == nil) {
					tbl.AddRow(tool.Name, tool.Type, tool.URL)
				}
			}
			tbl.Print()
		},
	}
	cmd.Flags().StringVarP(&toolType, "type", "t", "*", "Type of tool: web app┃desktop app┃bot┃gadget┃user script┃command line tool┃coding framework┃other|\"\"")
	return cmd
}

func NewToolHubToolsSearchCmd() *cobra.Command {
	var toolType string
	cmd := &cobra.Command{
		Use:     "search",
		Short:   "Search Toolhub Tools",
		Example: "search unicorn\nsearch upload --type=\"web app\"",
		Run: func(cmd *cobra.Command, args []string) {
			searchString := args[0]
			client := toolhub.NewClient()
			ctx := context.Background()
			tools, err := client.SearchTools(ctx, searchString, nil)
			if err != nil {
				color.Red("Error: %s", err)
				os.Exit(1)
			}

			headerFmt := color.New(color.FgGreen, color.Underline).SprintfFunc()
			columnFmt := color.New(color.FgYellow).SprintfFunc()

			tbl := table.New("Name", "Type", "URL")
			tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)

			for _, tool := range tools.Results {
				if toolType == "*" || toolType == tool.Type || (toolType == "" && tool.Type == nil) {
					tbl.AddRow(tool.Name, tool.Type, tool.URL)
				}
			}
			tbl.Print()
		},
	}
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
