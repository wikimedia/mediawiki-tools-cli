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


var mwddRedisCmd = &cobra.Command{
	Use:   "redis",
	Short: "Redis service",
	RunE:  nil,
}

var mwddRedisCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a Redis container",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		mwdd.DefaultForUser().UpDetached(
			[]string{"redis"},
			exec.HandlerOptions{
				Verbosity:   Verbosity,
			},
		)
	},
}

var mwddRedisDestroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy the Redis container and volumes",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity:   Verbosity,
		}
		mwdd.DefaultForUser().Rm( []string{"redis"},options)
		mwdd.DefaultForUser().RmVolumes( []string{"redis-data"},options)
	},
}

var mwddRedisSuspendCmd = &cobra.Command{
	Use:   "suspend",
	Short: "Suspend the Redis container",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity:   Verbosity,
		}
		mwdd.DefaultForUser().Stop( []string{"redis"},options)
	},
}

var mwddRedisResumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume the Redis container",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity:   Verbosity,
		}
		mwdd.DefaultForUser().Start( []string{"redis"},options)
	},
}

var mwddRedisExecCmd = &cobra.Command{
	Use:   "exec [flags] [command...]",
	Example:   "  exec bash\n  exec -- bash --help\n  exec --user root bash\n  exec --user root -- bash --help",
	Short: "Executes a command in the Redis container",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		mwdd.DefaultForUser().DockerExec(mwdd.DockerExecCommand{
			DockerComposeService: "redis",
			Command: args,
			User: User,
		})
	},
}

var mwddRedisCliCmd = &cobra.Command{
	Use:   "cli",
	Short: "Redis CLI for the container",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		mwdd.DefaultForUser().DockerExec(mwdd.DockerExecCommand{
			DockerComposeService: "redis",
			Command: []string{"redis-cli"},
		})
	},
}

func init() {
	mwddCmd.AddCommand(mwddRedisCmd)
	mwddRedisCmd.AddCommand(mwddRedisCreateCmd)
	mwddRedisCmd.AddCommand(mwddRedisDestroyCmd)
	mwddRedisCmd.AddCommand(mwddRedisSuspendCmd)
	mwddRedisCmd.AddCommand(mwddRedisResumeCmd)
	mwddRedisCmd.AddCommand(mwddRedisExecCmd)
	mwddRedisExecCmd.Flags().StringVarP(&User, "user", "u", mwdd.UserAndGroupForDockerExecution(), "User to run as, defaults to current OS user uid:gid")
	mwddRedisCmd.AddCommand(mwddRedisCliCmd)
}
