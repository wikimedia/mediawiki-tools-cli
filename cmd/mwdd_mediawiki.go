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

var mwddMediawikiCmd = &cobra.Command{
	Use:   "mediawiki",
	Short: "MediaWiki service",
	RunE:  nil,
}

var mwddMediawikiInstallCmd = &cobra.Command{
	Use:   "install [db name] [db type]",
	Short: "Installs a new MediaWiki site using install.php",
	Run: func(cmd *cobra.Command, args []string) {
		// TODO deal with errors from any of the below commands...
		dbname := "default"
		dbtype := "sqlite"
		if len(args) >= 1 {
			dbname = args[0]
		}
		if len(args) >= 2 {
			dbtype = args[1]
		}

		// Move custom LocalSetting.php so the install doesn't overwrite it
		mwdd.DefaultForUser().Exec("mediawiki",[]string{
			"mv",
			"/var/www/html/w/LocalSettings.php",
			"/var/www/html/w/LocalSettings.php.docker.tmp",
			}, exec.HandlerOptions{})

		// Do a DB type dependant install
		// TODO actually input the "right" settings so install.php output looks correct
		if dbtype == "sqlite" {
			mwdd.DefaultForUser().Exec("mediawiki",[]string{
				"php",
				"/var/www/html/w/maintenance/install.php",
				"--dbtype", dbtype,
				"--dbname", dbname,
				"--lang", "en",
				"--pass", "mwddpassword",
				"docker-" + dbname,
				"admin",
				}, exec.HandlerOptions{})
		}
		if dbtype == "mysql" {
			mwdd.DefaultForUser().Exec("mediawiki",[]string{
				"/wait-for-it.sh",
				"mysql:3306",
				}, exec.HandlerOptions{})
			mwdd.DefaultForUser().Exec("mediawiki",[]string{
				"php",
				"/var/www/html/w/maintenance/install.php",
				"--dbtype", dbtype,
				"--dbuser", "root",
				"--dbpass", "toor",
				"--dbname", dbname,
				"--dbserver", "mysql",
				"--lang", "en",
				"--pass", "mwddpassword",
				"docker-" + dbname,
				"admin",
				}, exec.HandlerOptions{})
		}

		// Move the auto generated LocalSetting.php somewhere else
		mwdd.DefaultForUser().Exec("mediawiki",[]string{
			"mv",
			"/var/www/html/w/LocalSettings.php",
			"/var/www/html/w/LocalSettings.php.docker.lastgenerated",
			}, exec.HandlerOptions{})

		// Move the custom one back
		mwdd.DefaultForUser().Exec("mediawiki",[]string{
			"mv",
			"/var/www/html/w/LocalSettings.php.docker.tmp",
			"/var/www/html/w/LocalSettings.php",
			}, exec.HandlerOptions{})

		// Run update.php once too
		mwdd.DefaultForUser().Exec("mediawiki",[]string{
			"php",
			"/var/www/html/w/maintenance/update.php",
			"--wiki", dbname,
			"--quick",
			}, exec.HandlerOptions{})
	},
}

var mwddMediawikiComposerCmd = &cobra.Command{
	Use:   "composer",
	Short: "Runs composer in a container in the context of MediaWiki",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Not yet implemented!");
	},
}

var mwddMediawikiCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create the Mediawiki containers",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity:   Verbosity,
		}
		// TODO mediawiki should come from some default definition set?
		mwdd.DefaultForUser().UpDetached( []string{"mediawiki"}, options )
		// TODO add functionality for writing to the hosts file...
		//mwdd.DefaultForUser().EnsureHostsFile()
	},
}

var mwddMediawikiDestroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy the Mediawiki containers",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity:   Verbosity,
		}
		mwdd.DefaultForUser().Rm( []string{"mediawiki"},options)
		mwdd.DefaultForUser().RmVolumes( []string{"mediawiki-data","mediawiki-images","mediawiki-logs"},options)
	},
}

var mwddMediawikiSuspendCmd = &cobra.Command{
	Use:   "suspend",
	Short: "Suspend the Mediawiki containers",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity:   Verbosity,
		}
		mwdd.DefaultForUser().Stop( []string{"mediawiki"},options)
	},
}

var mwddMediawikiResumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume the Mediawiki containers",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity:   Verbosity,
		}
		mwdd.DefaultForUser().Start( []string{"mediawiki"},options)
	},
}

var mwddMediawikiPhpunitCmd = &cobra.Command{
	Use:   "phpunit",
	Short: "Runs MediaWiki phpunit in the MediaWiki container",
	Run: func(cmd *cobra.Command, args []string) {
		wiki := "default"
		// TODO optionally take a --wiki (use default if not specified?) Maybe this should be done in LocalSettings?
		// if len(args) >= 1 {
		// 	wiki = args[0]
		// }
		mwdd.DefaultForUser().EnsureReady()
		mwdd.DefaultForUser().DockerExec(mwdd.DockerExecCommand{
			DockerComposeService: "mediawiki",
			Command: append([]string{"php", "/var/www/html/w/tests/phpunit/phpunit.php", "--wiki", wiki},args...),
		})
	},
}

var mwddMediawikiExecCmd = &cobra.Command{
	Use:   "exec [flags] [command...]",
	Example:   "  exec bash\n  exec -- bash --help\n  exec --user root bash\n  exec --user root -- bash --help",
	Short: "Executes a command in the MediaWiki container",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		mwdd.DefaultForUser().DockerExec(mwdd.DockerExecCommand{
			DockerComposeService: "mediawiki",
			Command: args,
			User: User,
		})
	},
}

func init() {
	mwddCmd.AddCommand(mwddMediawikiCmd)
	mwddMediawikiCmd.AddCommand(mwddMediawikiCreateCmd)
	mwddMediawikiCmd.AddCommand(mwddMediawikiDestroyCmd)
	mwddMediawikiCmd.AddCommand(mwddMediawikiSuspendCmd)
	mwddMediawikiCmd.AddCommand(mwddMediawikiResumeCmd)
	mwddMediawikiCmd.AddCommand(mwddMediawikiInstallCmd)
	mwddMediawikiCmd.AddCommand(mwddMediawikiComposerCmd)
	mwddMediawikiCmd.AddCommand(mwddMediawikiPhpunitCmd)
	mwddMediawikiCmd.AddCommand(mwddMediawikiExecCmd)
	mwddMediawikiExecCmd.Flags().StringVarP(&User, "user", "u", mwdd.UserAndGroupForDockerExecution(), "User to run as, defaults to current OS user uid:gid")

}
