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
	_ "embed"
	"os/exec"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/cli"
)

//go:embed long/mwdd_gerrit.md
var gerritLong string

func NewGerritCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "gerrit",
		Short: "Wikimedia Gerrit",
		Long:  cli.RenderMarkdown(gerritLong),
		RunE:  nil,
	}
}

// TODO factor this into a nice package / util?
func sshGerritCommand(args []string) *exec.Cmd {
	ssh := exec.Command("ssh", "-p", "29418", "gerrit.wikimedia.org", "gerrit")
	ssh.Args = append(ssh.Args, args...)
	return ssh
}

func gerritAttachToCmd(rootCmd *cobra.Command) {
	gerritCmd := NewGerritCmd()
	rootCmd.AddCommand(gerritCmd)

	gerritChangesCmd := NewGerritChangesCmd()
	gerritCmd.AddCommand(gerritChangesCmd)
	gerritChangesListCmd := NewGerritChangesListCmd()
	gerritChangesCmd.AddCommand(gerritChangesListCmd)

	gerritGroupCmd := NewGerritGroupCmd()
	gerritCmd.AddCommand(gerritGroupCmd)
	gerritGroupCmd.AddCommand(NewGerritGroupListCmd())
	gerritGroupCmd.AddCommand(NewGerritGroupSearchCmd())
	gerritGroupCmd.AddCommand(NewGerritGroupMembersCmd())

	gerritProjectCmd := NewGerritProjectCmd()
	gerritCmd.AddCommand(gerritProjectCmd)
	gerritProjectCmd.AddCommand(NewGerritProjectListCmd())
	gerritProjectCmd.AddCommand(NewGerritProjectSearchCmd())
	gerritProjectCmd.AddCommand(NewGerritProjectCurrentCmd())
}
