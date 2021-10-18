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
	"os"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/mwdd"
	"gitlab.wikimedia.org/releng/cli/internal/util/hosts"
)

var mwddHostsCmd = &cobra.Command{
	Use:   "hosts",
	Short: "Interact with your system hosts file",
	RunE:  nil,
}

var mwddHostsAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Adds development environment hosts into your system hosts file (might need sudo)",
	Run: func(cmd *cobra.Command, args []string) {
		changeResult := hosts.AddHosts(
			append(
				[]string{
					// TODO generate these by reading the yml files?
					"proxy.mwdd.localhost",
					"eventlogging.mwdd.localhost",
					"adminer.mwdd.localhost",
					"mailhog.mwdd.localhost",
					"graphite.mwdd.localhost",
					"phpmyadmin.mwdd.localhost",
					"default.mediawiki.mwdd.localhost",
				},
				mwdd.DefaultForUser().UsedHosts()...,
			),
		)
		handleChangeResult(changeResult)
	},
}

var mwddHostsRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Removes development environment hosts from your system hosts file (might need sudo)",
	Run: func(cmd *cobra.Command, args []string) {
		handleChangeResult(hosts.RemoveHostsWithSuffix("mwdd.localhost"))
	},
}

func handleChangeResult(result hosts.ChangeResult) {
	if result.Success && result.Altered {
		fmt.Println("Hosts file altered and updated: " + result.WriteFile)
	} else if result.Altered {
		fmt.Println("Wanted to alter your hosts file bu could not.")
		fmt.Println("You can re-run this command with sudo.")
		fmt.Println("Or edit the hosts file yourself.")
		fmt.Println("Temporary file: " + result.WriteFile)
		fmt.Println("")
		fmt.Println(result.Content)
	} else {
		fmt.Println("No changes needed.")
	}
}

var mwddHostsWritableCmd = &cobra.Command{
	Use:   "writable",
	Short: "Checks if you can write to the needed hosts file",
	Run: func(cmd *cobra.Command, args []string) {
		if hosts.Writable() {
			fmt.Println("Hosts file writable")
		} else {
			fmt.Println("Hosts file not writable")
			os.Exit(1)
		}
	},
}

func init() {
	mwddCmd.AddCommand(mwddHostsCmd)
	mwddHostsCmd.AddCommand(mwddHostsAddCmd)
	mwddHostsCmd.AddCommand(mwddHostsRemoveCmd)
	mwddHostsCmd.AddCommand(mwddHostsWritableCmd)
}
