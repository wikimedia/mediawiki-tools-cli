package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/cli"
	"gitlab.wikimedia.org/releng/cli/internal/updater"
)

func NewUpdateCmd() *cobra.Command {
	manualVersion := ""
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Checks for and performs updates",
		Run: func(cmd *cobra.Command, args []string) {
			if manualVersion == "" {
				canUpdate, toUpdateToOrMessage := updater.CanUpdate(VersionDetails.Version, VersionDetails.GitSummary)

				if !canUpdate {
					fmt.Println(toUpdateToOrMessage)
					os.Exit(0)
				}

				fmt.Println("New update found: " + toUpdateToOrMessage)
			} else {
				canMoveToVersion := updater.CanMoveToVersion(manualVersion)
				if !canMoveToVersion {
					fmt.Println("Can not find manual version " + manualVersion + " to move to")
					os.Exit(1)
				}
				fmt.Println("Updating to maually selected version: " + manualVersion)
			}

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

			// Start a progress bar
			updateProcessCompleted := false
			bar := progressbar.Default(100, "Updating binary")
			go func() {
				for !updateProcessCompleted {
					bar.Add(1)
					time.Sleep(100 * time.Millisecond)
				}
			}()

			// Perform the update
			var updateSuccess bool
			var updateMessage string
			if manualVersion == "" {
				// Technically there is a small race condition here, and we might update to a newer version if it was release between stages
				updateSuccess, updateMessage = updater.Update(VersionDetails.Version, VersionDetails.GitSummary)
			} else {
				updateSuccess, updateMessage = updater.MoveToVersion(manualVersion)
			}

			// Finish the progress bar
			updateProcessCompleted = true
			bar.Finish()
			fmt.Println("")

			// Output result
			fmt.Println(cli.RenderMarkdown(updateMessage))
			if !updateSuccess {
				os.Exit(1)
			}
		},
	}
	cmd.Flags().StringVarP(&manualVersion, "version", "", "", "Specific version to \"update\" to, or rollback to.")
	return cmd
}

func updateAttachToCmd(rootCmd *cobra.Command) {
	rootCmd.AddCommand(NewUpdateCmd())
}
