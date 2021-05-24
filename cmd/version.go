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
	"log"
	"os"

	"gerrit.wikimedia.org/r/mediawiki/tools/cli/internal/updater"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Output the version infomation",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf(`GitCommit: %s
GitBranch: %s
GitState: %s
GitSummary: %s
BuildDate: %s
Version: %s
`, GitCommit, GitBranch, GitState, GitSummary, BuildDate, Version)
	},
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Checks for and performs updates",
	Run: func(cmd *cobra.Command, args []string) {

		canUpdate, nextRelease := updater.CanUpdate(Version, GitSummary, Verbosity >= 2)
		if !canUpdate || nextRelease == nil {
			log.Println("No update available")
			os.Exit(0)
		}

		log.Println("New update found: " + nextRelease.Version.String())
		log.Println(nextRelease.AssetURL)

		updatePrompt := promptui.Prompt{
			Label:     " Do you want to update?",
			IsConfirm: true,
		}
		_, err := updatePrompt.Run()
		if err == nil {
			if(!updater.ShouldAllowUpdates( Version, GitSummary, Verbosity >= 2 )){
				log.Println("Can not update this version")
				os.Exit(1)
			}
			updater.UpdateTo(*nextRelease, Verbosity >= 2)
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(updateCmd)

	updateCmd.PersistentFlags().IntVarP(&Verbosity, "verbosity", "v", 1, "verbosity level (1-2)")
}
