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
	"os"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/cli"
	"gitlab.wikimedia.org/releng/cli/internal/exec"
	"gitlab.wikimedia.org/releng/cli/internal/mwdd"
)

//go:embed long/mwdd_mediawiki_quibble.md
var mwddMediawikiQuibbleLong string

func NewMediaWikiQuibbleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "quibble ...",
		Short: "Runs commands in a 'quibble' container.",
		Long:  cli.RenderMarkdown(mwddMediawikiQuibbleLong),
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				cmd.Help()
				os.Exit(1)
			}

			mwdd.DefaultForUser().EnsureReady()
			options := exec.HandlerOptions{
				Verbosity: globalOpts.Verbosity,
			}
			// With the current "abuse" of docker-compose to do this, we must run down before up incase the previous container failed?
			// Perhaps a better long term solution would be to NOT run up if a container exists with the correct name?
			// But then there is no way for this container to ever get updates
			// So a better solultion is to make all of this brittle
			mwdd.DefaultForUser().Rm([]string{"mediawiki-quibble"}, options)
			mwdd.DefaultForUser().UpDetached([]string{"mediawiki-quibble"}, options)
			command, env := mwdd.CommandAndEnvFromArgs(args)
			mwdd.DefaultForUser().DockerRun(applyRelevantMediawikiWorkingDirectory(mwdd.DockerExecCommand{
				DockerComposeService: "mediawiki-quibble",
				Command:              command,
				Env:                  env,
				User:                 User,
			}, "/workspace/src"))
		},
	}
	cmd.Flags().StringVarP(&User, "user", "u", mwdd.UserAndGroupForDockerExecution(), "User to run as, defaults to current OS user uid:gid")
	return cmd
}
