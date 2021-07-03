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


var mwddMySQLReplicaCmd = &cobra.Command{
	Use:   "mysql-replica",
	Short: "Sql replicated service",
	RunE:  nil,
}

var mwddMySQLReplicaCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create the MySQL Replication containers",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		mwdd.DefaultForUser().UpDetached(
			[]string{"mysql-replica","mysql-replica-configure-replication"},
			exec.HandlerOptions{
				Verbosity:   Verbosity,
			},
		)
	},
}

var mwddMySQLReplicaDestroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy the MySQL Replication containers and volumes",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity:   Verbosity,
		}
		mwdd.DefaultForUser().Rm( []string{"mysql-replica","mysql-replica-configure-replication"},options)
		mwdd.DefaultForUser().RmVolumes( []string{"mysql-replica-data"},options)
	},
}

var mwddMySQLReplicaSuspendCmd = &cobra.Command{
	Use:   "suspend",
	Short: "Suspend the MySQL Replication containers",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity:   Verbosity,
		}
		mwdd.DefaultForUser().Stop( []string{"mysql-replica","mysql-replica-configure-replication"},options)
	},
}

var mwddMySQLReplicaResumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume the MySQL Replication containers",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity:   Verbosity,
		}
		mwdd.DefaultForUser().Start( []string{"mysql-replica","mysql-replica-configure-replication"},options)
	},
}

var mwddMySQLReplicaExecCmd = &cobra.Command{
	Use:   "exec [flags] [command...]",
	Example:   "  exec bash\n  exec -- bash --help\n  exec --user root bash\n  exec --user root -- bash --help",
	Short: "Executes a command in the MySQL Replica container",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		mwdd.DefaultForUser().DockerExec(mwdd.DockerExecCommand{
			DockerComposeService: "mysql-replica",
			Command: args,
			User: User,
		})
	},
}

func init() {
	mwddCmd.AddCommand(mwddMySQLReplicaCmd)
	mwddMySQLReplicaCmd.AddCommand(mwddMySQLReplicaCreateCmd)
	mwddMySQLReplicaCmd.AddCommand(mwddMySQLReplicaDestroyCmd)
	mwddMySQLReplicaCmd.AddCommand(mwddMySQLReplicaSuspendCmd)
	mwddMySQLReplicaCmd.AddCommand(mwddMySQLReplicaResumeCmd)
	mwddMySQLReplicaCmd.AddCommand(mwddMySQLReplicaExecCmd)
	mwddMySQLReplicaExecCmd.Flags().StringVarP(&User, "user", "u", mwdd.UserAndGroupForDockerExecution(), "User to run as, defaults to current OS user uid:gid")
}
