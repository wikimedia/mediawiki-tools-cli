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

var mwddMediawikiFreshCmd = &cobra.Command{
	Use:   "fresh ...",
	Short: "Runs commands in a 'fresh' container.",
	Long: `Runs commands in a 'fresh' container.

	Various environment variables are already set up for you.
	- MW_SERVER=http://default.mediawiki.mwdd:${PORT}
	- MW_SCRIPT_PATH=/w
	- MEDIAWIKI_USER=Admin
	- MEDIAWIKI_PASSWORD=mwddpassword

	Note: the lack of .localhost at the end of the site name. Using .localhost will NOT work in this container.`,
	Example: `        # Start an interactive terminal in the fresh container
	fresh bash
  
	# Run npm ci in the currently directory (if within mediawiki)
	fresh npm ci                                    # Run npm ci in the current directory (if within mediawiki)
  
	# Run mediawiki core tests (when in the mediawiki core directory)
	fresh npm run selenium-test
  
	# Run a single Wikibase extension test spec (when in the Wikibase extension directory)
	fresh npm run selenium-test:repo -- -- --spec repo/tests/selenium/specs/item.js`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Help()
			os.Exit(1)
		}

		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity: Verbosity,
		}
		mwdd.DefaultForUser().UpDetached([]string{"mediawiki-fresh"}, options)
		mwdd.DefaultForUser().DockerRun(applyRelevantWorkingDirectory(mwdd.DockerExecCommand{
			DockerComposeService: "mediawiki-fresh",
			Command:              args,
			User:                 User,
		}))
	},
}

func init() {
	mwddMediawikiCmd.AddCommand(mwddMediawikiFreshCmd)
	mwddMediawikiFreshCmd.Flags().StringVarP(&User, "user", "u", mwdd.UserAndGroupForDockerExecution(), "User to run as, defaults to current OS user uid:gid")
}
