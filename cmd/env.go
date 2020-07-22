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
	"fmt"

	"gerrit.wikimedia.org/r/mediawiki/tools/cli/internal/env"

	"github.com/spf13/cobra"
)

var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Provides subcommands for interacting with development environment variables",
	RunE:  nil,
}

var deleteCmd = &cobra.Command{
	Use:   "delete [name]",
	Short: "Deletes an environment variable",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		env.Delete(args[0])
	},
}

var setCmd = &cobra.Command{
	Use:   "set [name] [value]",
	Short: "Set an environment variable",
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		env.Set(args[0], args[1])
	},
}

var getCmd = &cobra.Command{
	Use:   "get [name]",
	Short: "Get an environment variable",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(env.Get(args[0]))
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all environment variables",
	Run: func(cmd *cobra.Command, args []string) {
		for name, value := range env.List() {
			fmt.Println(name + "=" + value)
		}
	},
}

var whereCmd = &cobra.Command{
	Use:   "where",
	Short: "Output the location of the .env file",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(env.GetPath())
	},
}

func init() {
	dockerCmd.AddCommand(envCmd)

	envCmd.AddCommand(whereCmd)
	envCmd.AddCommand(setCmd)
	envCmd.AddCommand(getCmd)
	envCmd.AddCommand(listCmd)
	envCmd.AddCommand(deleteCmd)
}
