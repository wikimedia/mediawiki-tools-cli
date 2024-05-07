package update

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/updater"
)

func NewUpdateCmd() *cobra.Command {
	versionInput := ""
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Checks for and performs updates",
		Example: `update
update --version=0.10 --no-interaction
update --version=https://gitlab.wikimedia.org/repos/releng/cli/-/jobs/252738/artifacts/download`,
		Run: func(cmd *cobra.Command, args []string) {
			currentVersion := cli.VersionDetails.Version
			var targetVersion string
			var targetArtifact string

			// No manual version, so genrally check for new releases
			if versionInput == "" {
				canUpdate, toUpdateToOrMessage := updater.CanUpdate(currentVersion, cli.VersionDetails.GitSummary)

				if !canUpdate {
					fmt.Println(toUpdateToOrMessage)
					os.Exit(0)
				}

				fmt.Println("New update found: " + toUpdateToOrMessage)
				targetVersion = toUpdateToOrMessage
			}

			// Manual version is specified, so check it
			if versionInput != "" {
				// if manual version looks like a URL, we will just try and download it later
				if versionInput[:4] == "http" {
					fmt.Println("Downloading from URL: " + versionInput)
					targetArtifact = versionInput
				} else {
					canMoveToVersion := updater.CanMoveToVersion(versionInput)
					if !canMoveToVersion {
						fmt.Println("Can not find manual version " + versionInput + " to move to")
						os.Exit(1)
					}
					fmt.Println("Updating to manually selected version: " + versionInput)
					targetVersion = versionInput
				}
			}

			// If we are in interactive mode, confirm the user wants to continue with the update
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
			bar := progressbar.Default(111, "Updating binary")
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
			// Either from a Gitlab release
			if targetVersion != "" {
				updateSuccess, _ = updater.MoveToVersion(targetVersion)
			}
			// Or from a Gitlab build artifact
			if targetArtifact != "" {
				tempDownloadFile, err := updater.DownloadFile(targetArtifact)
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
				err = os.Chmod(os.Args[0], 0o755)
				if err != nil {
					// Switch them back
					os.Rename(oldFileName, os.Args[0])
					fmt.Println(err)
					os.Exit(1)
				}
			}

			// Finish the progress bar
			updateProcessCompleted = true
			err := bar.Finish()
			if err != nil {
				fmt.Println(err)
			}

			// Exit with 1 if we didn't update
			if !updateSuccess {
				os.Exit(1)
			}

			// Figure out what to tell the user about the update

			// Output changelog of the versions we are moving between
			if targetVersion != "" {
				releasesUpdatedThrough, err := updater.RelengCliGetReleasesBetweenTags(currentVersion, targetVersion)
				if err != nil {
					fmt.Printf("Could not fetch changelog between versions: %s\n", err)
					// TODO link to releases page
				} else {
					fmt.Print("\nChanges between versions:\n\n")
					for _, release := range releasesUpdatedThrough {
						desc := strings.Trim(release.Description, "\r\n")
						// TODO Remove any lines that start with "CHANGELOG extracted from"
						formatted := strings.Trim(cli.RenderMarkdown(desc), "\r\n")
						fmt.Print(formatted)
					}
				}
			}
			if targetArtifact != "" {
				fmt.Println("Updated from a build artifact")
				// TODO link
			}
		},
	}
	cmd.Flags().StringVarP(&versionInput, "version", "", "", "Specific version to \"update\" to, or rollback to.")
	return cmd
}
