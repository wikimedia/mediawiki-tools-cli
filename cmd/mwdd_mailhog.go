/*Package cmd is used for command line.

Copyright © 2020 Addshore

This program is free software: you can mailhogtribute it and/or modify
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

var mwddMailhogCmd = &cobra.Command{
	Use:   "mailhog",
	Short: "Mailhog service",
	Long: `Mailhog service

Using this will automagically configure $wgSMTP for MediaWiki

$wgSMTP = [
    'host'     => 'mailhog',
    'IDHost'   => 'mailhog',
    'port'     => '8025',
    'auth'     => false,
];`,
	RunE: nil,
}

var mwddMailhogCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a Mailhog container",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		mwdd.DefaultForUser().UpDetached(
			[]string{"mailhog"},
			exec.HandlerOptions{
				Verbosity: globalOpts.Verbosity,
			},
		)
	},
}

var mwddMailhogDestroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy the Mailhog container and volumes",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity: globalOpts.Verbosity,
		}
		mwdd.DefaultForUser().Rm([]string{"mailhog"}, options)
	},
}

var mwddMailhogSuspendCmd = &cobra.Command{
	Use:   "suspend",
	Short: "Suspend the Mailhog container",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity: globalOpts.Verbosity,
		}
		mwdd.DefaultForUser().Stop([]string{"mailhog"}, options)
	},
}

var mwddMailhogResumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume the Mailhog container",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity: globalOpts.Verbosity,
		}
		mwdd.DefaultForUser().Start([]string{"mailhog"}, options)
	},
}

var mwddMailhogExecCmd = &cobra.Command{
	Use:     "exec [flags] [command...]",
	Example: "  exec bash\n  exec -- bash --help\n  exec --user root bash\n  exec --user root -- bash --help",
	Short:   "Executes a command in the Mailhog container",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		command, env := mwdd.CommandAndEnvFromArgs(args)
		mwdd.DefaultForUser().DockerExec(mwdd.DockerExecCommand{
			DockerComposeService: "mailhog",
			Command:              command,
			Env:                  env,
			User:                 User,
		})
	},
}

func init() {
	mwddCmd.AddCommand(mwddMailhogCmd)
	mwddMailhogCmd.AddCommand(mwddMailhogCreateCmd)
	mwddMailhogCmd.AddCommand(mwddMailhogDestroyCmd)
	mwddMailhogCmd.AddCommand(mwddMailhogSuspendCmd)
	mwddMailhogCmd.AddCommand(mwddMailhogResumeCmd)
	mwddMailhogCmd.AddCommand(mwddMailhogExecCmd)
	mwddMailhogExecCmd.Flags().StringVarP(&User, "user", "u", mwdd.UserAndGroupForDockerExecution(), "User to run as, defaults to current OS user uid:gid")
}
