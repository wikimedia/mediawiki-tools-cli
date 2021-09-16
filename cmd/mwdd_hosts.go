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

	"gerrit.wikimedia.org/r/mediawiki/tools/cli/internal/mwdd"
	"gerrit.wikimedia.org/r/mediawiki/tools/cli/internal/util/hosts"
	"github.com/spf13/cobra"
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
		save := hosts.AddHosts(
			append(
				[]string{
					// TODO generate these by reading the yml files?
					"proxy.mwdd.localhost",
					"adminer.mwdd.localhost",
					"graphite.mwdd.localhost",
					"phpmyadmin.mwdd.localhost",
					"default.mediawiki.mwdd.localhost",
				},
				mwdd.DefaultForUser().UsedHosts()...,
			),
		)
		if save.Success {
			fmt.Println("Hosts file updated!")
		} else {
			fmt.Println("Could not save your hosts file.")
			fmt.Println("You can return with sudo.")
			fmt.Println("Or edit the hosts fiel yourself.")
			fmt.Println("Temporary file: " + save.TmpFile)
			fmt.Println("")
			fmt.Println(save.Content)
		}
	},
}

var mwddHostsRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Removes development environment hosts from your system hosts file (might need sudo)",
	Run: func(cmd *cobra.Command, args []string) {
		save := hosts.RemoveHostsWithSuffix("mwdd.localhost")
		if save.Success {
			fmt.Println("Hosts file updated!")
		} else {
			fmt.Println("Could not save your hosts file.")
			fmt.Println("You can return with sudo.")
			fmt.Println("Or edit the hosts fiel yourself.")
			fmt.Println("Temporary file: " + save.TmpFile)
			fmt.Println("")
			fmt.Println(save.Content)
		}
	},
}

func init() {
	mwddCmd.AddCommand(mwddHostsCmd)
	mwddHostsCmd.AddCommand(mwddHostsAddCmd)
	mwddHostsCmd.AddCommand(mwddHostsRemoveCmd)
}
