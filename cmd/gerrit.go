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
	"os/exec"

	"github.com/spf13/cobra"
)

var mwddGerritCmd = &cobra.Command{
	Use:   "gerrit",
	Short: "Wikimedia Gerrit",
	Long: `Wikimedia Gerrit

Your ssh config must be setup to connect you to gerrit.wikimedia.org already`,
	RunE: nil,
}

// TODO factor this into a nice package / util?
func sshGerritCommand(args []string) *exec.Cmd {
	ssh := exec.Command("ssh", "-p", "29418", "gerrit.wikimedia.org", "gerrit")
	ssh.Args = append(ssh.Args, args...)
	return ssh
}

func init() {
	rootCmd.AddCommand(mwddGerritCmd)
}
