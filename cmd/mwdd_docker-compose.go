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

	"gerrit.wikimedia.org/r/mediawiki/tools/cli/internal/exec"
	"gerrit.wikimedia.org/r/mediawiki/tools/cli/internal/mwdd"
	"github.com/spf13/cobra"
)

var mwddDockerComposeCmd = &cobra.Command{
	Use:   "docker-compose",
	RunE:  nil,
}

var mwddDockerComposeRawCmd = &cobra.Command{
	Use:   "raw",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Not yet implemented!");
	},
}

var mwddDockerComposePsCmd = &cobra.Command{
	Use:   "ps",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		mwdd.DefaultForUser().DockerCompose(
			mwdd.DockerComposeCommand{
				Command: "ps",
				HandlerOptions: exec.HandlerOptions{
					Verbosity:   Verbosity,
				},
			},
		)
	},
}

func init() {
	mwddCmd.AddCommand(mwddDockerComposeCmd)
	mwddDockerComposeCmd.AddCommand(mwddDockerComposeRawCmd)
	mwddDockerComposeCmd.AddCommand(mwddDockerComposePsCmd)
}
