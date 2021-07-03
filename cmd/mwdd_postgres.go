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


var mwddPostgresCmd = &cobra.Command{
	Use:   "postgres",
	Short: "Postgres service",
	RunE:  nil,
}

var mwddPostgresCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create the Postgres containers",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		mwdd.DefaultForUser().UpDetached(
			[]string{"postgres"},
			exec.HandlerOptions{
				Verbosity:   Verbosity,
			},
		)
	},
}

var mwddPostgresDestroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy the Postgres containers and volumes",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity:   Verbosity,
		}
		mwdd.DefaultForUser().Rm( []string{"postgres"},options)
		mwdd.DefaultForUser().RmVolumes( []string{"postgres-data"},options)
	},
}

var mwddPostgresSuspendCmd = &cobra.Command{
	Use:   "suspend",
	Short: "Suspend the Postgres containers",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity:   Verbosity,
		}
		mwdd.DefaultForUser().Stop( []string{"postgres"},options)
	},
}

var mwddPostgresResumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume the Postgres containers",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity:   Verbosity,
		}
		mwdd.DefaultForUser().Start( []string{"postgres"},options)
	},
}

var mwddPostgresExecCmd = &cobra.Command{
	Use:   "exec [flags] [command...]",
	Example:   "  exec bash\n  exec -- bash --help\n  exec --user root bash\n  exec --user root -- bash --help",
	Short: "Executes a command in the Postgres container",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		mwdd.DefaultForUser().DockerExec(mwdd.DockerExecCommand{
			DockerComposeService: "postgres",
			Command: args,
			User: User,
		})
	},
}

func init() {
	mwddCmd.AddCommand(mwddPostgresCmd)
	mwddPostgresCmd.AddCommand(mwddPostgresCreateCmd)
	mwddPostgresCmd.AddCommand(mwddPostgresDestroyCmd)
	mwddPostgresCmd.AddCommand(mwddPostgresSuspendCmd)
	mwddPostgresCmd.AddCommand(mwddPostgresResumeCmd)
	mwddPostgresCmd.AddCommand(mwddPostgresExecCmd)
	mwddPostgresExecCmd.Flags().StringVarP(&User, "user", "u", mwdd.UserAndGroupForDockerExecution(), "User to run as, defaults to current OS user uid:gid")
}
