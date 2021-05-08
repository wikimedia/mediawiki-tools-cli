/*Package cmd is used for building command line commands

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

/*Env top level env command*/
func Env(Short string) *cobra.Command {
	return  &cobra.Command{
		Use:   "env",
		Short: Short,
		RunE:  nil,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Do nothing, but override any other PersistentPreRuns
		},
	}
}

/*EnvDelete env delete command*/
func EnvDelete(directory func()string) *cobra.Command {
	return  &cobra.Command{
		Use:   "delete [name]",
		Short: "Deletes an environment variable",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			env.DotFileForDirectory(directory()).Delete(args[0])
		},
	}
}

/*EnvSet env set command*/
func EnvSet(directory func()string) *cobra.Command {
	return  &cobra.Command{
		Use:   "set [name] [value]",
		Short: "Set an environment variable",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			env.DotFileForDirectory(directory()).Set(args[0], args[1])
		},
	}
}

/*EnvGet env get command*/
func EnvGet(directory func()string) *cobra.Command {
	return  &cobra.Command{
		Use:   "get [name]",
		Short: "Get an environment variable",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(env.DotFileForDirectory(directory()).Get(args[0]))
		},
	}
}

/*EnvList env list command*/
func EnvList(directory func()string) *cobra.Command {
	return  &cobra.Command{
		Use:   "list",
		Short: "List all environment variables",
		Run: func(cmd *cobra.Command, args []string) {
			for name, value := range env.DotFileForDirectory(directory()).List() {
				fmt.Println(name + "=" + value)
			}
		},
	}
}

/*EnvWhere env where command*/
func EnvWhere(directory func()string) *cobra.Command {
	return  &cobra.Command{
		Use:   "where",
		Short: "Output the location of the .env file",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(env.DotFileForDirectory(directory()).Path())
		},
	}
}