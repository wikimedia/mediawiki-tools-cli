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
	"github.com/profclems/glab/commands"
	"github.com/profclems/glab/commands/cmdutils"
	"github.com/profclems/glab/pkg/glinstance"
)

func init() {
	cmdFactory := cmdutils.NewFactory()

	glinstance.OverrideDefault("gitlab.wikimedia.org")

	// Try to keep this version in line with the addshore fork for now...
	rootCmd.AddCommand(commands.NewCmdRoot(cmdFactory, "mwcli", "1.20-addshore-test-004"))
}
