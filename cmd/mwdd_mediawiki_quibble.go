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
	"os"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/exec"
	"gitlab.wikimedia.org/releng/cli/internal/mwdd"
)

var mwddMediawikiQuibbleCmd = &cobra.Command{
	Use:   "quibble ...",
	Short: "Runs commands in a 'quibble' container.",
	Long: `Runs commands in a 'quibble' container.
	
	https://doc.wikimedia.org/quibble/
	
	THis integration is WORK IN PROGRESS`,
	Example: `        # Start an interactive terminal in the quibble container
	quibble bash
  
	# Get help for the quibble CLI tool
	quibble quibble -- --help

	# Run php-unit quibble stage using your mwdd LocalSettings.php, skipping anything that alters your installation
	quibble quibble -- --skip-zuul --skip-deps --skip-install --db-is-external --run phpunit-unit

	# Run composer phpunit:unit inside the quibble container
	quibble quibble -- --skip-zuul --skip-deps --skip-install --db-is-external --command "composer phpunit:unit"

    Gotchas:
        - This is a WORK IN PROGRESS integration, so don't expect all quibble features to work.
        - quibble will run tests for ALL checked out extensions by default.
        - If you let quibble touch your setup (missing --skip-install for example) it might break your environment.
		- quibble has various things hardcoded :(, for example the user and password for browser tests, you might find the below command helpful.

	mw docker mediawiki exec php maintenance/CeateAndPromote.php -- --sysop WikiAdmin testwikijenkinspass`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Help()
			os.Exit(1)
		}

		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity: Verbosity,
		}
		mwdd.DefaultForUser().UpDetached([]string{"mediawiki-quibble"}, options)
		mwdd.DefaultForUser().DockerRun(applyRelevantMediawikiWorkingDirectory(mwdd.DockerExecCommand{
			DockerComposeService: "mediawiki-quibble",
			Command:              args,
			User:                 User,
		}, "/workspace/src"))
	},
}

func init() {
	mwddMediawikiCmd.AddCommand(mwddMediawikiQuibbleCmd)
	mwddMediawikiQuibbleCmd.Flags().StringVarP(&User, "user", "u", mwdd.UserAndGroupForDockerExecution(), "User to run as, defaults to current OS user uid:gid")
}
