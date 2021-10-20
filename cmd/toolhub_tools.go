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
	"encoding/json"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/rodaine/table"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/toolhub"
)

var toolhubToolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "Interact with Toolhub tools",
}

func NewToolHubToolsListCmd() *cobra.Command {
	var toolType string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Toolhub Tools",
		Example: `  list
  list --type="web app"
  list --type=""`,
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
		Use:   "search",
		Short: "Search Toolhub Tools",
		Example: `  search unicorn
  search upload --type="web app"`,
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

var toolhubToolsGetCmd = &cobra.Command{
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

func init() {
	toolhubCmd.AddCommand(toolhubToolsCmd)
	toolhubToolsCmd.AddCommand(NewToolHubToolsListCmd())
	toolhubToolsCmd.AddCommand(NewToolHubToolsSearchCmd())
	toolhubToolsCmd.AddCommand(toolhubToolsGetCmd)
}
