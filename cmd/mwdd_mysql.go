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
	"gerrit.wikimedia.org/r/mediawiki/tools/cli/internal/exec"
	"gerrit.wikimedia.org/r/mediawiki/tools/cli/internal/mwdd"
	"github.com/spf13/cobra"
)


var mwddMySQLCmd = &cobra.Command{
	Use:   "mysql",
	Short: "Sql service",
	RunE:  nil,
}

var mwddMySQLCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create the MySQL containers",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		mwdd.DefaultForUser().UpDetached(
			[]string{"mysql","mysql-configure-replication"},
			exec.HandlerOptions{
				Verbosity:   Verbosity,
			},
		)
	},
}

var mwddMySQLDestroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy the MySQL containers and volumes",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity:   Verbosity,
		}
		mwdd.DefaultForUser().Rm( []string{"mysql","mysql-configure-replication"},options)
		mwdd.DefaultForUser().RmVolumes( []string{"mysql-data","mysql-configure-replication-data"},options)
	},
}

var mwddMySQLSuspendCmd = &cobra.Command{
	Use:   "suspend",
	Short: "Suspend the MySQL containers",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity:   Verbosity,
		}
		mwdd.DefaultForUser().Stop( []string{"mysql","mysql-configure-replication"},options)
	},
}

var mwddMySQLResumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume the MySQL containers",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity:   Verbosity,
		}
		mwdd.DefaultForUser().Start( []string{"mysql","mysql-configure-replication"},options)
	},
}

var mwddMySQLExecCmd = &cobra.Command{
	Use:   "exec [flags] [command...]",
	Example:   "  exec bash\n  exec -- bash --help\n  exec --user root bash\n  exec --user root -- bash --help",
	Short: "Executes a command in the MySQL container",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		mwdd.DefaultForUser().DockerExec(mwdd.DockerExecCommand{
			DockerComposeService: "mysql",
			Command: args,
			User: User,
		})
	},
}

func init() {
	mwddCmd.AddCommand(mwddMySQLCmd)
	mwddMySQLCmd.AddCommand(mwddMySQLCreateCmd)
	mwddMySQLCmd.AddCommand(mwddMySQLDestroyCmd)
	mwddMySQLCmd.AddCommand(mwddMySQLSuspendCmd)
	mwddMySQLCmd.AddCommand(mwddMySQLResumeCmd)
	mwddMySQLCmd.AddCommand(mwddMySQLExecCmd)
	mwddMySQLExecCmd.Flags().StringVarP(&User, "user", "u", mwdd.UserAndGroupForDockerExecution(), "User to run as, defaults to current OS user uid:gid")
}
