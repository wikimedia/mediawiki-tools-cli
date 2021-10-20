/*Package cmd is used for command line.

Copyright © 2020 Addshore

This program is free software: you can eventloggingtribute it and/or modify
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

var mwddEventloggingCmd = &cobra.Command{
	Use: "eventlogging",
	Aliases: []string{
		"eventgate",
	},
	Short: "Eventlogging service",
	Long: `Eventlogging service

Checkout the logs of this service in order to see events coming in.

You probably want to have the following extensions enabled for eventlogging to function.

wfLoadExtensions( [
    'EventBus',
    'EventStreamConfig',
    'EventLogging',
    'WikimediaEvents'
  ] );

Using this will automagically configure a eventlogging server for MediaWiki.

$wgEventServices = [
	'*' => [ 'url' => 'http://eventlogging:8192/v1/events' ],
];
$wgEventServiceDefault = '*';
$wgEventLoggingStreamNames = false;
$wgEventLoggingServiceUri = "http://eventlogging.mwdd.localhost:" . parse_url($wgServer)['port'] . "/v1/events";
$wgEventLoggingQueueLingerSeconds = 1;
$wgEnableEventBus = defined( "MW_PHPUNIT_TEST" ) ? "TYPE_NONE" : "TYPE_ALL";`,
	RunE: nil,
}

var mwddEventloggingCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a Eventlogging container",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		mwdd.DefaultForUser().UpDetached(
			[]string{"eventlogging"},
			exec.HandlerOptions{
				Verbosity: globalOpts.Verbosity,
			},
		)
	},
}

var mwddEventloggingDestroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy the Eventlogging container and volumes",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity: globalOpts.Verbosity,
		}
		mwdd.DefaultForUser().Rm([]string{"eventlogging"}, options)
	},
}

var mwddEventloggingSuspendCmd = &cobra.Command{
	Use:   "suspend",
	Short: "Suspend the Eventlogging container",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity: globalOpts.Verbosity,
		}
		mwdd.DefaultForUser().Stop([]string{"eventlogging"}, options)
	},
}

var mwddEventloggingResumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume the Eventlogging container",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity: globalOpts.Verbosity,
		}
		mwdd.DefaultForUser().Start([]string{"eventlogging"}, options)
	},
}

var mwddEventloggingExecCmd = &cobra.Command{
	Use:     "exec [flags] [command...]",
	Example: "  exec bash\n  exec -- bash --help\n  exec --user root bash\n  exec --user root -- bash --help",
	Short:   "Executes a command in the Eventlogging container",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		command, env := mwdd.CommandAndEnvFromArgs(args)
		mwdd.DefaultForUser().DockerExec(mwdd.DockerExecCommand{
			DockerComposeService: "eventlogging",
			Command:              command,
			Env:                  env,
			User:                 User,
		})
	},
}

func init() {
	mwddCmd.AddCommand(mwddEventloggingCmd)
	mwddEventloggingCmd.AddCommand(mwddEventloggingCreateCmd)
	mwddEventloggingCmd.AddCommand(mwddEventloggingDestroyCmd)
	mwddEventloggingCmd.AddCommand(mwddEventloggingSuspendCmd)
	mwddEventloggingCmd.AddCommand(mwddEventloggingResumeCmd)
	mwddEventloggingCmd.AddCommand(mwddEventloggingExecCmd)
	mwddEventloggingExecCmd.Flags().StringVarP(&User, "user", "u", mwdd.UserAndGroupForDockerExecution(), "User to run as, defaults to current OS user uid:gid")
}
