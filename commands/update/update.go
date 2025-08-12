package update

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmdgloss"
	gitlabb "gitlab.wikimedia.org/repos/releng/cli/internal/gitlab"
	"gitlab.wikimedia.org/repos/releng/cli/internal/updater"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
)

func Cmd() *cobra.Command {
	versionInput := ""
	dryRun := false
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Checks for and performs updates",
		Example: cobrautil.NormalizeExample(`update
update --version=v0.10.0 --no-interaction
update --version=https://gitlab.wikimedia.org/repos/releng/cli/-/jobs/252738/artifacts/download`),
		Run: func(cmd *cobra.Command, args []string) {
			currDetails := cli.VersionDetails
			var targetVersion cli.Version
			var targetArtifact string

			// No manual version, so generally check for new releases
			if versionInput == "" {
				targetRelease, err := gitlabb.RelengCliLatestRelease()
				if err != nil {
					// TODO allow err of not found?
					logrus.Error(fmt.Errorf("could not fetch latest release link: %s", err))
					os.Exit(1)
				}

				targetVersion = cli.VersionFromUserInput(targetRelease.TagName)

				// Make sure we are not already on the latest version
				if currDetails.Version == targetVersion {
					cmd.Println("You are already on the latest version: " + targetVersion.String())
					os.Exit(0)
				}

				// TODO actually do semver comparison
				// But this probably doesn't matter right now, as we generally only have 1 stream of released going at a time...

				targetReleaseLink, err := gitlabb.RelengCliReleaseBinary(targetVersion.Tag())
				if err != nil {
					// TODO allow err of not found?
					logrus.Error(fmt.Errorf("could not fetch release binary link: %s", err))
					os.Exit(1)
				}

				cmd.Println("New update found: " + targetVersion.String())
				cmd.Println("Release URL: " + targetVersion.ReleasePage())
				cmd.Println("Artifact URL: " + targetArtifact)
				targetArtifact = targetReleaseLink.DirectAssetURL
			} else {
				// Manual version is URL?
				if len(versionInput) >= 4 && versionInput[:4] == "http" {
					// TODO if we can auto detect a gitlab build, link to that too
					cmd.Println("Artifact URL: " + versionInput)
					targetArtifact = versionInput
				} else {
					// Probably gitlab version of tag
					targetVersion = cli.VersionFromUserInput(versionInput)

					targetReleaseLink, err := gitlabb.RelengCliReleaseBinary(targetVersion.Tag())
					if err != nil {
						// TODO, allow err of not found?
						logrus.Error(fmt.Errorf("could not fetch release binary link: %s", err))
						os.Exit(1)
					}

					cmd.Println("Updating to manually selected version: " + targetVersion.String())
					cmd.Println("Release URL: " + targetVersion.ReleasePage())
					cmd.Println("Artifact URL: " + targetReleaseLink.DirectAssetURL)
					targetArtifact = targetReleaseLink.DirectAssetURL
				}
			}

			// If we are in interactive mode, confirm the user wants to continue with the update
			if !cli.Opts.NoInteraction && !dryRun {
				response := false
				prompt := &survey.Confirm{
					Message: "Do you want to update?",
				}
				err := survey.AskOne(prompt, &response)
				if err != nil {
					cmd.Println(err)
					os.Exit(1)
				}
				if !response {
					cmd.Println("Update cancelled")
					os.Exit(0)
				}
			}

			// Perform the update
			var newMwFileLocation string
			if !dryRun {
				response, err := updater.DownloadFileResponse(targetArtifact)
				if err != nil {
					cmd.Println(err)
					os.Exit(1)
				}
				defer response.Body.Close()

				if response.ContentLength <= 0 {
					logrus.Warn("Could not parse content length, so download progress display may be broken")
				}

				tempDirDownload, tempDirDownloadCloser := tmpDir("mwcli-update-download")
				defer tempDirDownloadCloser()
				tempDownloadFile, err := os.CreateTemp(tempDirDownload, "download-*.tmp")
				if err != nil {
					logrus.Error(fmt.Errorf("could not create temp file for download: %s", err))
					os.Exit(1)
				}
				defer tempDownloadFile.Close()
				tempDownloadFilePath := tempDownloadFile.Name()

				var p *tea.Program
				pw := &cmdgloss.ProgressWriter{
					Total:  int(response.ContentLength),
					File:   tempDownloadFile,
					Reader: response.Body,
					OnProgress: func(ratio float64) {
						p.Send(cmdgloss.ProgressMsg(ratio))
					},
				}
				m := cmdgloss.Model{
					Pw:       pw,
					Progress: progress.New(progress.WithDefaultGradient()),
				}
				p = tea.NewProgram(m)
				go pw.Start() // Start the download
				if _, err := p.Run(); err != nil {
					cmd.Println("error with progress bar:", err)
					os.Exit(1)
				}
				if int64(pw.Downloaded) < response.ContentLength {
					logrus.Error("Download seems incomplete.")
					os.Exit(1)
				}

				// Is the file a zip?
				if updater.IsZipFile(tempDownloadFilePath) {
					tempDir, tempDirCloser := tmpDir("mwcli-update-extract")
					defer tempDirCloser()

					logrus.Trace("Unzipping downloaded file: " + tempDownloadFilePath)
					err = updater.Unzip(tempDownloadFilePath, tempDir)
					if err != nil {
						logrus.Error("could not unzip the downloaded file: "+tempDownloadFilePath, err)
						os.Exit(1)
					}
					// it should contain a dir called bin, and in that a file called mw
					newMwFileLocation = tempDir + "/bin/mw"
				} else {
					newMwFileLocation = tempDownloadFilePath
				}

				// Make sure it exists or error
				if _, err := os.Stat(newMwFileLocation); os.IsNotExist(err) {
					logrus.Error(fmt.Errorf("could not find the mw binary in the downloaded file: %s", newMwFileLocation))
					os.Exit(1)
				}

				executablePath, err := os.Executable()
				if err != nil {
					logrus.Error(fmt.Errorf("could not get the current executable path: %s", err))
					os.Exit(1)
				}
				executableName := executablePath[strings.LastIndex(executablePath, "/")+1:]
				logrus.Trace("Current executable name: " + executableName)
				logrus.Trace("Current executable path: " + executablePath)

				// Make a copy of the current binary, in a temp location
				tempCopyName := executableName + ".update-copy." + time.Now().Format("2006-01-02-15-04-05")
				// Get a full path in the temporary dir
				tempDir, tempDirCloser := tmpDir("mwcli-update-backup")
				defer tempDirCloser()
				tempCopyPath := tempDir + "/" + tempCopyName

				// Copy the current binary to a temp location
				_, err = copyFile(executablePath, tempCopyPath)
				if err != nil {
					logrus.Error(fmt.Errorf("could not backup current binary: %s", err))
					os.Exit(1)
				}
				defer os.Remove(tempCopyPath)

				// Move the new file to the desired location
				_, err = copyFile(newMwFileLocation, executablePath)
				if err != nil {
					logrus.Error(fmt.Errorf("could not move new binary to location: %s", err))
					// Switch them back
					copyFile(tempCopyPath, executablePath)
					os.Exit(1)
				}
				defer os.Remove(newMwFileLocation)

				// Make sure it is executable
				// TODO only do this, if it wasn't already executable?
				err = os.Chmod(executablePath, 0o755)
				if err != nil {
					logrus.Error(fmt.Errorf("could not make new binary executable: %s", err))
					// Switch them back.
					// TODO: This wont restore the +x, will it?
					copyFile(tempCopyPath, executablePath)
					os.Exit(1)
				}
			} else {
				cmd.Println("Dry run, no actual update performed")
				if targetVersion != "" {
					cmd.Println("Would have updated to version: " + targetVersion + " (using Gitlab releases)")
				}
				if targetArtifact != "" {
					cmd.Println("Would have updated from build artifact: " + targetArtifact + " (using Gitlab CI artifacts)")
				}
			}

			cmd.Println("Update successful")

			// Output changelog of the versions we are moving between
			if targetVersion != "" {
				// If the versions are the same, nothing changes
				if targetVersion == currDetails.Version {
					cmd.Println("No changes between versions")
					os.Exit(0)
				}

				releasesUpdatedThrough, err := updater.RelengCliGetReleasesBetweenTags(currDetails.Version.Tag(), targetVersion.Tag())
				if err != nil {
					logrus.Error(fmt.Errorf("could not fetch changelog between versions: %s", err))
					cmd.Println("You can try running the following command to see the last version's changelog:")
					cmd.Println("  " + targetVersion.ReleaseNotesCommand())
					cmd.Println("Or view the changelog online:")
					cmd.Println("  " + targetVersion.ReleasePage())
				} else {
					cmd.Print("\nChanges between versions:\n\n")
					for _, release := range releasesUpdatedThrough {
						desc := strings.Trim(release.Description, "\r\n")
						// TODO Remove any lines that start with "CHANGELOG extracted from"
						formatted := strings.Trim(cli.RenderMarkdown(desc), "\r\n")
						cmd.Println(formatted)
					}
				}
			}
		},
	}
	cmd.Flags().StringVarP(&versionInput, "version", "", "", "Specific version to \"update\" to, or rollback to.")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "", false, "Show what would be updated, but don't actually update.")
	return cmd
}

func copyFile(in, out string) (int64, error) {
	logrus.Trace("Copying file from: " + in + " to: " + out)
	i, e := os.Open(in)
	if e != nil {
		return 0, e
	}
	defer i.Close()
	o, e := os.Create(out)
	if e != nil {
		return 0, e
	}
	defer o.Close()
	return o.ReadFrom(i)
}

func tmpDir(name string) (string, func()) {
	tempDir, err := os.MkdirTemp(os.TempDir(), name)
	if err != nil {
		logrus.Error(fmt.Errorf("could not create temp dir for %s: %s", name, err))
		os.Exit(1)
	}
	logrus.Trace("Created temp dir: " + tempDir)
	return tempDir, func() {
		os.RemoveAll(tempDir)
		logrus.Trace("Removed temp dir: " + tempDir)
	}
}
