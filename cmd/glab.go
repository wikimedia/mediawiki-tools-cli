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
	"runtime/debug"
	"strings"

	"github.com/profclems/glab/commands"
	"github.com/profclems/glab/commands/cmdutils"
	"github.com/profclems/glab/pkg/glinstance"
)

func init() {
	cmdFactory := cmdutils.NewFactory()

	glinstance.OverrideDefault("gitlab.wikimedia.org")

	// Try to keep this version in line with the addshore fork for now...
	glabCommand := commands.NewCmdRoot(cmdFactory, "mwcli "+glabVersion(), BuildDate)
	glabCommand.Short = "Wikimedia Gitlab instance"
	glabCommand.Use = strings.Replace(glabCommand.Use, "glab", "gitlab", 1)
	glabCommand.Aliases = []string{"glab"}

	// Hide various built in glab commands
	toHide := []string{
		// glab does not need to be updated itself, instead mwcli would need to be updated
		"check-update",
		// issues will not be used on the Wikimedia gitlab instance
		"issue",
	}
	for _, command := range glabCommand.Commands() {
		_, found := findInSlice(toHide, command.Name())
		if found {
			glabCommand.RemoveCommand(command)
		}
		// TODO fix this one upsteam
		if command.Name() == "config" {
			command.Long = strings.Replace(command.Long, "https://gitlab.com", "https://gitlab.wikimedia.org", -1)
		}
	}

	rootCmd.AddCommand(glabCommand)
}

func findInSlice(slice []string, val string) (int, bool) {
	for i, item := range slice {
		if item == val {
			return i, true
		}
	}
	return -1, false
}

func glabVersion() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		panic("couldn't read build info")
	}

	for _, v := range bi.Deps {
		if v.Path == "github.com/profclems/glab" {
			return v.Version
		}
	}

	return "unknown"
}
