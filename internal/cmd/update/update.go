package update

import (
	"fmt"
	"os"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/updater"
)

func NewUpdateCmd() *cobra.Command {
	manualVersion := ""
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Checks for and performs updates",
		Example: `update
update --version=0.10 --no-interaction
update --version=https://gitlab.wikimedia.org/repos/releng/cli/-/jobs/252738/artifacts/download`,
		Run: func(cmd *cobra.Command, args []string) {
			// No manual version, so genrally check for new releases
			if manualVersion == "" {
				canUpdate, toUpdateToOrMessage := updater.CanUpdate(cli.VersionDetails.Version, cli.VersionDetails.GitSummary)

				if !canUpdate {
					fmt.Println(toUpdateToOrMessage)
					os.Exit(0)
				}

				fmt.Println("New update found: " + toUpdateToOrMessage)
			}

			// Manual version is specified, so check it
			if manualVersion != "" {
				// if manual version looks like a URL, we will just try and download it later
				if manualVersion[:4] == "http" {
					fmt.Println("Downloading from URL: " + manualVersion)
				} else {
					canMoveToVersion := updater.CanMoveToVersion(manualVersion)
					if !canMoveToVersion {
						fmt.Println("Can not find manual version " + manualVersion + " to move to")
						os.Exit(1)
					}
					fmt.Println("Updating to manually selected version: " + manualVersion)
				}
			}

			if !cli.Opts.NoInteraction {
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
					err := bar.Add(1)
					if err != nil {
						fmt.Println(err)
					}
					time.Sleep(100 * time.Millisecond)
				}
			}()

			// Perform the update
			var updateSuccess bool
			var updateMessage string
			if manualVersion == "" {
				// Technically there is a small race condition here, and we might update to a newer version if it was release between stages
				updateSuccess, updateMessage = updater.Update(cli.VersionDetails.Version, cli.VersionDetails.GitSummary)
			} else {
				if manualVersion[:4] == "http" {
					tempDownloadFile, err := updater.DownloadFile(manualVersion)
					if err != nil {
						fmt.Println(err)
						os.Exit(1)
					}
					// Extract tempFile which is a zip file
					tempDir, err := os.MkdirTemp("", "mwcli-update")
					if err != nil {
						fmt.Println(err)
						os.Exit(1)
					}
					defer os.RemoveAll(tempDir)

					err = updater.Unzip(tempDownloadFile, tempDir)
					if err != nil {
						fmt.Println("Could not unzip the downloaded file: " + tempDownloadFile)
						os.Exit(1)
					}
					// it should contain a dir called bin, and in that a file called mw
					newMwFileLocation := tempDir + "/bin/mw"

					// Make sure it exists or error
					if _, err := os.Stat(newMwFileLocation); os.IsNotExist(err) {
						fmt.Println("Could not find the mw binary in the downloaded file: " + newMwFileLocation)
						os.Exit(1)
					}

					// Move the current bin to a temp location
					oldFileName := os.Args[0] + ".old." + time.Now().Format("2006-01-02-15-04-05")
					err = os.Rename(os.Args[0], oldFileName)
					if err != nil {
						fmt.Println(err)
						os.Exit(1)
					}
					// defer deletion of this file
					defer os.Remove(oldFileName)

					// Move the new file to the current location
					err = os.Rename(newMwFileLocation, os.Args[0])
					if err != nil {
						// Switch them back
						os.Rename(oldFileName, os.Args[0])
						fmt.Println(err)
						os.Exit(1)
					}

					// Make sure it is executable
					err = os.Chmod(os.Args[0], 0755)
					if err != nil {
						// Switch them back
						os.Rename(oldFileName, os.Args[0])
						fmt.Println(err)
						os.Exit(1)
					}
				} else {
					updateSuccess, updateMessage = updater.MoveToVersion(manualVersion)
				}
			}

			// Finish the progress bar
			updateProcessCompleted = true
			err := bar.Finish()
			if err != nil {
				fmt.Println(err)
			}

			// Output result, if we are moving to a real release
			if manualVersion[:4] != "http" {
				fmt.Println("")
				fmt.Println(cli.RenderMarkdown(updateMessage))
			}

			if !updateSuccess {
				os.Exit(1)
			}
		},
	}
	cmd.Flags().StringVarP(&manualVersion, "version", "", "", "Specific version to \"update\" to, or rollback to.")
	return cmd
}
