/*Package cmd is used for command line.

Copyright © 2020 Addshore

This program is free software: you can memcachedtribute it and/or modify
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
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/exec"
	"gitlab.wikimedia.org/releng/cli/internal/mwdd"
)

var mwddMemcachedCmd = &cobra.Command{
	Use:   "memcached",
	Short: "Memcached service",
	Long: `Memcached service

Using this will automagically configure a memcached server for MediaWiki

$wgMemCachedServers = [ 'memcached:11211' ];`,
	RunE: nil,
}

var mwddMemcachedCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a Memcached container",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		mwdd.DefaultForUser().UpDetached(
			[]string{"memcached"},
			exec.HandlerOptions{
				Verbosity: globalOpts.Verbosity,
			},
		)
	},
}

var mwddMemcachedDestroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy the Memcached container and volumes",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity: globalOpts.Verbosity,
		}
		mwdd.DefaultForUser().Rm([]string{"memcached"}, options)
	},
}

var mwddMemcachedSuspendCmd = &cobra.Command{
	Use:   "suspend",
	Short: "Suspend the Memcached container",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity: globalOpts.Verbosity,
		}
		mwdd.DefaultForUser().Stop([]string{"memcached"}, options)
	},
}

var mwddMemcachedResumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume the Memcached container",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity: globalOpts.Verbosity,
		}
		mwdd.DefaultForUser().Start([]string{"memcached"}, options)
	},
}

var mwddMemcachedExecCmd = &cobra.Command{
	Use:     "exec [flags] [command...]",
	Example: "  exec bash\n  exec -- bash --help\n  exec --user root bash\n  exec --user root -- bash --help",
	Short:   "Executes a command in the Memcached container",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		mwdd.DefaultForUser().DockerExec(mwdd.DockerExecCommand{
			DockerComposeService: "memcached",
			Command:              args,
			User:                 User,
		})
	},
}

func init() {
	mwddCmd.AddCommand(mwddMemcachedCmd)
	mwddMemcachedCmd.AddCommand(mwddMemcachedCreateCmd)
	mwddMemcachedCmd.AddCommand(mwddMemcachedDestroyCmd)
	mwddMemcachedCmd.AddCommand(mwddMemcachedSuspendCmd)
	mwddMemcachedCmd.AddCommand(mwddMemcachedResumeCmd)
	mwddMemcachedCmd.AddCommand(mwddMemcachedExecCmd)
	mwddMemcachedExecCmd.Flags().StringVarP(&User, "user", "u", mwdd.UserAndGroupForDockerExecution(), "User to run as, defaults to current OS user uid:gid")
}
