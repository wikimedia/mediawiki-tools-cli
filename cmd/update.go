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

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/updater"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Checks for and performs updates",
	Run: func(cmd *cobra.Command, args []string) {
		canUpdate, toUpdateToOrMessage := updater.CanUpdate(Version, GitSummary, Verbosity >= 2)

		if !canUpdate {
			fmt.Println(toUpdateToOrMessage)
			os.Exit(0)
		}

		fmt.Println("New update found: " + toUpdateToOrMessage)

		if !NoInteraction {
			updatePrompt := promptui.Prompt{
				Label:     " Do you want to update?",
				IsConfirm: true,
			}
			_, err := updatePrompt.Run()
			if err != nil {
				return
			}
		}

		// Technically there is a small race condition here, and we might update to a newer version if it was release between stages
		updateSuccess, updateMessage := updater.Update(Version, GitSummary, Verbosity >= 2)
		fmt.Println(updateMessage)
		if !updateSuccess {
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)

	updateCmd.PersistentFlags().IntVarP(&Verbosity, "verbosity", "v", 1, "verbosity level (1-2)")
	updateCmd.PersistentFlags().BoolVarP(&NoInteraction, "no-interaction", "n", false, "Do not ask any interactive question")
}
