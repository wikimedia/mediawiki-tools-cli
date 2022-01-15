/*Package cmd is used for command line.

Copyright © 2020 Addshore

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Output the version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("mw version", VersionDetails.Version, "(", VersionDetails.BuildDate, ")")
		fmt.Println("https://gitlab.wikimedia.org/releng/cli/-/releases")

		if globalOpts.Verbosity > 1 {
			fmt.Printf(`GitCommit: %s
GitBranch: %s
GitState: %s
GitSummary: %s
BuildDate: %s
Version: %s
`,
				VersionDetails.GitCommit,
				VersionDetails.GitBranch,
				VersionDetails.GitState,
				VersionDetails.GitSummary,
				VersionDetails.BuildDate,
				VersionDetails.Version,
			)
		}
	},
}

func versionAttachToCmd(rootCmd *cobra.Command) {
	rootCmd.AddCommand(versionCmd)
}
