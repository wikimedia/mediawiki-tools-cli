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
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/updater"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Checks for and performs updates",
	Run: func(cmd *cobra.Command, args []string) {
		canUpdate, toUpdateToOrMessage := updater.CanUpdate(VersionDetails.Version, VersionDetails.GitSummary, globalOpts.Verbosity >= 2)

		if !canUpdate {
			fmt.Println(toUpdateToOrMessage)
			os.Exit(0)
		}

		fmt.Println("New update found: " + toUpdateToOrMessage)

		if !globalOpts.NoInteraction {
			response := false
			prompt := &survey.Confirm{
				Message: "Do you want to update?",
			}
			err := survey.AskOne(prompt, &response)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			if !response {
				fmt.Println("Update cancelled")
				os.Exit(0)
			}
		}

		// Technically there is a small race condition here, and we might update to a newer version if it was release between stages
		updateSuccess, updateMessage := updater.Update(VersionDetails.Version, VersionDetails.GitSummary, globalOpts.Verbosity >= 2)
		fmt.Println(updateMessage)
		if !updateSuccess {
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
