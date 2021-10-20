/*Package cmd is used for command line.

Copyright © 2020 Addshore

This program is free software: you can elasticsearchtribute it and/or modify
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

var mwddElasticsearchCmd = &cobra.Command{
	Use:   "elasticsearch",
	Short: "Elasticsearch service",
	Long: `Elasticsearch service

Using this will automagically configure a elasticsearch server for MediaWiki via the CirrusSearch extension.
In order for this to do anything you will need to CirrusSearch extension installed and enabled.

$wgCirrusSearchServers = [ 'elasticsearch' ];

In order to configure a search index for a wiki, you'll need to run some maintenance scripts:

# Configure the search index and populate it with content
php extensions/CirrusSearch/maintenance/UpdateSearchIndexConfig.php
php extensions/CirrusSearch/maintenance/ForceSearchIndex.php --skipLinks --indexOnSkip
php extensions/CirrusSearch/maintenance/ForceSearchIndex.php --skipParse

# And you'll need to process the job queue any time you add/update content and want it updated in ElasticSearch
php maintenance/runJobs.php
`,
	RunE: nil,
}

var mwddElasticsearchCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a Elasticsearch container",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		mwdd.DefaultForUser().UpDetached(
			[]string{"elasticsearch"},
			exec.HandlerOptions{
				Verbosity: globalOpts.Verbosity,
			},
		)
	},
}

var mwddElasticsearchDestroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy the Elasticsearch container and volumes",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity: globalOpts.Verbosity,
		}
		mwdd.DefaultForUser().Rm([]string{"elasticsearch"}, options)
		mwdd.DefaultForUser().RmVolumes([]string{"elasticsearch-data"}, options)
	},
}

var mwddElasticsearchSuspendCmd = &cobra.Command{
	Use:   "suspend",
	Short: "Suspend the Elasticsearch container",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity: globalOpts.Verbosity,
		}
		mwdd.DefaultForUser().Stop([]string{"elasticsearch"}, options)
	},
}

var mwddElasticsearchResumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume the Elasticsearch container",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity: globalOpts.Verbosity,
		}
		mwdd.DefaultForUser().Start([]string{"elasticsearch"}, options)
	},
}

var mwddElasticsearchExecCmd = &cobra.Command{
	Use:     "exec [flags] [command...]",
	Example: "  exec bash\n  exec -- bash --help\n  exec --user root bash\n  exec --user root -- bash --help",
	Short:   "Executes a command in the Elasticsearch container",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		command, env := mwdd.CommandAndEnvFromArgs(args)
		mwdd.DefaultForUser().DockerExec(mwdd.DockerExecCommand{
			DockerComposeService: "elasticsearch",
			Command:              command,
			Env:                  env,
			User:                 User,
		})
	},
}

func init() {
	mwddCmd.AddCommand(mwddElasticsearchCmd)
	mwddElasticsearchCmd.AddCommand(mwddElasticsearchCreateCmd)
	mwddElasticsearchCmd.AddCommand(mwddElasticsearchDestroyCmd)
	mwddElasticsearchCmd.AddCommand(mwddElasticsearchSuspendCmd)
	mwddElasticsearchCmd.AddCommand(mwddElasticsearchResumeCmd)
	mwddElasticsearchCmd.AddCommand(mwddElasticsearchExecCmd)
	mwddElasticsearchExecCmd.Flags().StringVarP(&User, "user", "u", mwdd.UserAndGroupForDockerExecution(), "User to run as, defaults to current OS user uid:gid")
}
