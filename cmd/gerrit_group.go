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
	cmdutil "gitlab.wikimedia.org/releng/cli/internal/util/cmd"
	stringsutil "gitlab.wikimedia.org/releng/cli/internal/util/strings"
)

var mwddGerritGroupCmd = &cobra.Command{
	Use:   "group",
	Short: "Interact with Gerrit groups",
}

var mwddGerritGroupListCmd = &cobra.Command{
	Use:   "list",
	Short: "List Gerrit groups",
	Run: func(cmd *cobra.Command, args []string) {
		ssh := cmdutil.AttachAllIO(sshGerritCommand([]string{"ls-groups"}))
		if err := ssh.Run(); err != nil {
			os.Exit(1)
		}
	},
}

var mwddGerritGroupSearchCmd = &cobra.Command{
	Use:   "search [search string]...",
	Short: "Search Gerrit groups",
	Args:  cobra.MinimumNArgs(1),
	Example: `  search wmde
  search extension Wikibase`,
	Run: func(cmd *cobra.Command, args []string) {
		ssh := cmdutil.AttachInErrIO(sshGerritCommand([]string{"ls-groups"}))
		out := cmdutil.AttachOutputBuffer(ssh)

		if err := ssh.Run(); err != nil {
			os.Exit(1)
		}

		fmt.Println(stringsutil.FilterMultiline(out.String(), args))
	},
}

var mwddGerritGroupMembersCmd = &cobra.Command{
	Use:   "members [group name]",
	Short: "List members of a Gerrit group",
	Args:  cobra.MinimumNArgs(1),
	Example: `  members wmde
  members mediawiki`,
	Run: func(cmd *cobra.Command, args []string) {
		ssh := cmdutil.AttachAllIO(sshGerritCommand([]string{"ls-members", args[0]}))
		if err := ssh.Run(); err != nil {
			os.Exit(1)
		}
	},
}

func init() {
	mwddGerritCmd.AddCommand(mwddGerritGroupCmd)
	mwddGerritGroupCmd.AddCommand(mwddGerritGroupListCmd)
	mwddGerritGroupCmd.AddCommand(mwddGerritGroupSearchCmd)
	mwddGerritGroupCmd.AddCommand(mwddGerritGroupMembersCmd)
}
