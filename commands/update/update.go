package update

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmdgloss"
	gitlabb "gitlab.wikimedia.org/repos/releng/cli/internal/gitlab"
	"gitlab.wikimedia.org/repos/releng/cli/internal/releaseversion"
	"gitlab.wikimedia.org/repos/releng/cli/internal/updater"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
	"golang.org/x/term"
)

func Cmd() *cobra.Command {
	versionInput := ""
	dryRun := false
	force := false
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Checks for and performs updates",
		Example: cobrautil.NormalizeExample(`update
update --version=v0.10.0 --no-interaction
update --force
update --version=nightly
update --version=https://gitlab.wikimedia.org/repos/releng/cli/-/jobs/252738/artifacts/download`),
		RunE: func(cmd *cobra.Command, args []string) error {
			currDetails := cli.VersionDetails
			var targetVersion cli.Version
			var targetArtifact string

			// No manual version, so generally check for new releases
			if versionInput == "" {
				targetRelease, err := gitlabb.RelengCliLatestRelease()
				if err != nil {
					// TODO allow err of not found?
					return fmt.Errorf("could not fetch latest release link: %s", err)
				}

				targetVersion = cli.VersionFromUserInput(targetRelease.TagName)

				// Reinstall only with --force; automatic selection never downgrades.
				if !automaticUpdateNeeded(currDetails.Version, targetVersion, force) {
					cmd.PrintErrf("Installed version %s is already at or above the latest stable release %s.\n", currDetails.Version, targetVersion)
					return nil
				}

				targetReleaseLink, err := gitlabb.RelengCliReleaseBinary(targetVersion.Tag())
				if err != nil {
					// TODO allow err of not found?
					return fmt.Errorf("could not fetch release binary link: %s", err)
				}

				cmd.PrintErrln("New update found: " + targetVersion.String())
				cmd.PrintErrln("Release URL: " + targetVersion.ReleasePage())
				targetArtifact = targetReleaseLink.DirectAssetURL
				cmd.PrintErrln("Artifact URL: " + targetArtifact)
			} else if versionInput == "nightly" {
				var err error
				targetArtifact, err = gitlabb.RelengCliNightlyBinaryURL()
				if err != nil {
					return err
				}
				cmd.PrintErrln("Updating to the latest successful nightly build")
				cmd.PrintErrln("Artifact URL: " + targetArtifact)
			} else {
				// Manual version is URL or local file path?
				isURL := len(versionInput) >= 4 && (versionInput[:4] == "http" || (len(versionInput) >= 7 && versionInput[:7] == "file://"))
				isLocalFile := len(versionInput) > 0 && (versionInput[0] == '/' || (len(versionInput) > 1 && versionInput[1] == ':')) // Unix path or Windows path

				if isURL || isLocalFile {
					// TODO if we can auto detect a gitlab build, link to that too
					cmd.PrintErrln("Artifact URL: " + versionInput)
					targetArtifact = versionInput
				} else {
					// Probably gitlab version of tag
					targetVersion = cli.VersionFromUserInput(versionInput)

					targetReleaseLink, err := gitlabb.RelengCliReleaseBinary(targetVersion.Tag())
					if err != nil {
						// TODO, allow err of not found?
						return fmt.Errorf("could not fetch release binary link: %s", err)
					}

					cmd.PrintErrln("Updating to manually selected version: " + targetVersion.String())
					cmd.PrintErrln("Release URL: " + targetVersion.ReleasePage())
					cmd.PrintErrln("Artifact URL: " + targetReleaseLink.DirectAssetURL)
					targetArtifact = targetReleaseLink.DirectAssetURL
				}
			}

			// If we are in interactive mode, confirm the user wants to continue with the update
			if !cli.Opts.NoInteraction && !dryRun {
				response := false
				prompt := &survey.Confirm{
					Message: "Do you want to update?",
				}
				err := survey.AskOne(prompt, &response, survey.WithStdio(os.Stdin, os.Stderr, os.Stderr))
				if err != nil {
					return err
				}
				if !response {
					cmd.PrintErrln("Update cancelled")
					return nil
				}
			}

			// Perform the update
			var newMwFileLocation string
			if !dryRun {
				response, err := updater.DownloadFileResponse(targetArtifact)
				if err != nil {
					return fmt.Errorf("could not download update: %w", err)
				}
				defer response.Body.Close()

				if response.ContentLength <= 0 {
					slog.Debug("update response has no content length")
				}

				tempDirDownload, tempDirDownloadCloser, err := tmpDir("mwcli-update-download")
				if err != nil {
					return err
				}
				defer tempDirDownloadCloser()
				tempDownloadFile, err := os.CreateTemp(tempDirDownload, "download-*.tmp")
				if err != nil {
					return fmt.Errorf("could not create temp file for download: %s", err)
				}
				defer tempDownloadFile.Close()
				tempDownloadFilePath := tempDownloadFile.Name()

				// Check if we have a TTY for progress bar display
				isTTY := term.IsTerminal(int(os.Stderr.Fd()))

				if isTTY {
					// Show progress bar only in TTY environments
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
					p = tea.NewProgram(m, tea.WithOutput(cmd.ErrOrStderr()))
					go pw.Start() // Start the download
					if _, err := p.Run(); err != nil {
						return fmt.Errorf("download progress: %w", err)
					}
					if int64(pw.Downloaded) < response.ContentLength {
						return fmt.Errorf("download seems incomplete")
					}
				} else {
					// No TTY available, just copy directly without progress bar
					_, err := tempDownloadFile.ReadFrom(response.Body)
					if err != nil {
						return fmt.Errorf("could not download file: %s", err)
					}
					if response.ContentLength > 0 {
						// Get the actual file size to verify download
						fileInfo, err := tempDownloadFile.Stat()
						if err != nil {
							return fmt.Errorf("could not get temp file size: %s", err)
						}
						if fileInfo.Size() < response.ContentLength {
							return fmt.Errorf("download seems incomplete")
						}
					}
				}

				// Is the file a zip?
				isZip, err := updater.IsZipFile(tempDownloadFilePath)
				if err != nil {
					return err
				}
				if isZip {
					tempDir, tempDirCloser, err := tmpDir("mwcli-update-extract")
					if err != nil {
						return err
					}
					defer tempDirCloser()

					slog.Debug("Unzipping downloaded file: " + tempDownloadFilePath)
					err = updater.Unzip(tempDownloadFilePath, tempDir)
					if err != nil {
						return fmt.Errorf("unzip update: %w", err)
					}
					newMwFileLocation, err = archiveBinaryPath(tempDir, runtime.GOOS, runtime.GOARCH)
					if err != nil {
						return fmt.Errorf("select executable from update archive: %w", err)
					}
				} else {
					newMwFileLocation = tempDownloadFilePath
				}

				// Make sure it exists or error
				if _, err := os.Stat(newMwFileLocation); os.IsNotExist(err) {
					return fmt.Errorf("could not find the executable in the downloaded file: %s", newMwFileLocation)
				}

				executablePath, err := getExecutablePath()
				if err != nil {
					return fmt.Errorf("could not get the current executable path: %s", err)
				}

				executableName := executableNameFromPath(executablePath)
				slog.Debug("Current executable name: " + executableName)
				slog.Debug("Current executable path: " + executablePath)

				// Make a copy of the current binary, in a temp location
				tempCopyName := executableName + ".update-copy." + time.Now().Format("2006-01-02-15-04-05")
				// Get a full path in the temporary dir
				tempDir, tempDirCloser, err := tmpDir("mwcli-update-backup")
				if err != nil {
					return err
				}
				defer tempDirCloser()
				tempCopyPath := filepath.Join(tempDir, tempCopyName)

				// Copy the current binary to a temp location
				_, err = copyFile(executablePath, tempCopyPath)
				if err != nil {
					return fmt.Errorf("could not backup current binary: %s", err)
				}
				defer os.Remove(tempCopyPath)

				if runtime.GOOS == "windows" {
					err = scheduleWindowsReplacement(newMwFileLocation, executablePath)
					if err != nil {
						return fmt.Errorf("could not schedule Windows replacement: %s", err)
					}
					cmd.PrintErrln("Update staged. Please run the command again in a moment to use the new version.")
					return nil
				}

				// On non-Windows systems, avoid writing directly to the currently-running
				// executable (can fail with ETXTBSY). Stage a file in the destination
				// directory and atomically rename it into place.
				err = replaceExecutableNonWindows(newMwFileLocation, executablePath)
				if err != nil {
					return fmt.Errorf("install update: %w", err)
				}
				defer os.Remove(newMwFileLocation)

				// Make sure it is executable
				// TODO only do this, if it wasn't already executable?
				err = os.Chmod(executablePath, 0o755)
				if err != nil {
					_, restoreErr := copyFile(tempCopyPath, executablePath)
					if restoreErr != nil {
						return fmt.Errorf("make update executable: %v; restore backup: %w", err, restoreErr)
					}
					return fmt.Errorf("make update executable: %w", err)
				}
			} else {
				cmd.PrintErrln("Dry run, no actual update performed")
				if targetVersion != "" {
					cmd.PrintErrln("Would have updated to version: " + targetVersion + " (using Gitlab releases)")
				}
				if targetArtifact != "" {
					cmd.PrintErrln("Would have updated from build artifact: " + targetArtifact + " (using Gitlab CI artifacts)")
				}
			}

			if dryRun {
				return nil
			}
			cmd.PrintErrln("Update successful")

			if targetVersion != "" {
				cmd.PrintErrln(describeVersionTransition(currDetails.Version, targetVersion))
			}

			// Output changelog of the versions we are moving between
			if targetVersion != "" {
				// If the versions are the same, nothing changes
				if targetVersion == currDetails.Version {
					cmd.Println("No changes between versions")
					return nil
				}

				releasesUpdatedThrough, err := updater.RelengCliGetReleasesBetweenTags(currDetails.Version.Tag(), targetVersion.Tag())
				if err != nil {
					cmd.PrintErrf("Warning: could not fetch release notes for transition %s -> %s: %v\n", currDetails.Version, targetVersion, err)
					cmd.Println("You can try running the following command to see the last version's changelog:")
					cmd.Println("  " + strings.Replace(targetVersion.ReleaseNotesCommand(), cli.DefaultCommandName+" ", cmd.Root().Name()+" ", 1))
					cmd.Println("Or view the changelog online:")
					cmd.Println("  " + targetVersion.ReleasePage())
				} else {
					cmd.Printf("\nRelease notes for transition %s -> %s:\n\n", currDetails.Version, targetVersion)
					if len(releasesUpdatedThrough) == 0 {
						cmd.Println("No release notes to display for this version transition.")
					}
					for _, release := range releasesUpdatedThrough {
						desc := strings.Trim(release.Description, "\r\n")
						// TODO Remove any lines that start with "CHANGELOG extracted from"
						formatted := strings.Trim(cli.RenderMarkdown(desc), "\r\n")
						cmd.Println(formatted)
						cmd.Println()
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&versionInput, "version", "", "", "Release version, nightly, artifact URL, or local executable path")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "", false, "Show what would be updated, but don't actually update.")
	cmd.Flags().BoolVarP(&force, "force", "", false, "Reinstall even if already on the latest version")
	return cmd
}

func scheduleWindowsReplacement(srcPath, destPath string) error {
	escape := func(s string) string {
		return strings.ReplaceAll(s, "'", "''")
	}

	script := fmt.Sprintf("$src='%s'; $dst='%s'; for ($i=0; $i -lt 100; $i++) { try { Copy-Item -LiteralPath $src -Destination $dst -Force; Remove-Item -LiteralPath $src -Force -ErrorAction SilentlyContinue; exit 0 } catch { Start-Sleep -Milliseconds 200 } }; exit 1", escape(srcPath), escape(destPath))

	command := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script)
	return command.Start()
}

func copyFile(in, out string) (int64, error) {
	slog.Debug("Copying file from: " + in + " to: " + out)
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

func tmpDir(name string) (string, func(), error) {
	tempDir, err := os.MkdirTemp(os.TempDir(), name)
	if err != nil {
		return "", nil, fmt.Errorf("create temporary directory for %s: %w", name, err)
	}
	slog.Debug("Created temp dir: " + tempDir)
	return tempDir, func() {
		os.RemoveAll(tempDir)
		slog.Debug("Removed temp dir: " + tempDir)
	}, nil
}

// getExecutablePath returns a reliable path to the current executable.
// On Linux, it uses /proc/self/exe which is more reliable than os.Executable()
// in containerized environments. Falls back to os.Executable() on other systems.
func getExecutablePath() (string, error) {
	// On Linux, try /proc/self/exe first (more reliable in containers)
	procExe := "/proc/self/exe"
	if fi, err := os.Lstat(procExe); err == nil && (fi.Mode()&os.ModeSymlink) != 0 {
		// It's a symlink, resolve it
		target, err := os.Readlink(procExe)
		if err == nil && target != "" {
			slog.Debug("Using /proc/self/exe: " + target)
			return target, nil
		}
	}

	// Fall back to os.Executable()
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}

	// Try to resolve symlinks using filepath.EvalSymlinks for a more stable result
	resolvedPath, err := filepath.EvalSymlinks(execPath)
	if err == nil && resolvedPath != "" {
		slog.Debug("Using resolved executable path: " + resolvedPath)
		return resolvedPath, nil
	}

	// If resolution fails, use the original path
	slog.Debug("Using os.Executable path: " + execPath)
	return execPath, nil
}

func executableNameFromPath(executablePath string) string {
	idx := strings.LastIndexAny(executablePath, "/\\")
	if idx == -1 {
		return executablePath
	}
	if idx+1 >= len(executablePath) {
		return executablePath
	}
	return executablePath[idx+1:]
}

func replaceExecutableNonWindows(newPath, destPath string) error {
	// Fast path: same filesystem rename directly into place.
	if err := os.Rename(newPath, destPath); err == nil {
		return nil
	} else {
		slog.Debug("Direct os.Rename failed, using same-directory staged replacement: " + err.Error())
	}

	destDir := filepath.Dir(destPath)
	stagedFile, err := os.CreateTemp(destDir, ".mw-update-staged-*")
	if err != nil {
		return err
	}
	stagedPath := stagedFile.Name()
	if err := stagedFile.Close(); err != nil {
		_ = os.Remove(stagedPath)
		return err
	}

	_, err = copyFile(newPath, stagedPath)
	if err != nil {
		_ = os.Remove(stagedPath)
		return err
	}

	if err := os.Chmod(stagedPath, 0o755); err != nil {
		_ = os.Remove(stagedPath)
		return err
	}

	if err := os.Rename(stagedPath, destPath); err != nil {
		_ = os.Remove(stagedPath)
		return err
	}

	return nil
}

// Explicit --version selection bypasses this guard and may downgrade. --force
// permits reinstalling the current release, but never an automatic downgrade.
func automaticUpdateNeeded(current, target cli.Version, force bool) bool {
	if current == target {
		return force
	}
	from, fromErr := releaseversion.Parse(current.String())
	to, toErr := releaseversion.Parse(target.String())
	if fromErr != nil || toErr != nil {
		// Development builds have no release ordering; allow moving to stable.
		return true
	}
	comparison := from.Compare(to)
	return comparison < 0 || (comparison == 0 && force)
}

func describeVersionTransition(from, to cli.Version) string {
	if from == to {
		return "Version unchanged: " + from.String()
	}

	fromSemver, fromErr := releaseversion.Parse(from.String())
	toSemver, toErr := releaseversion.Parse(to.String())
	if fromErr == nil && toErr == nil {
		cmp := fromSemver.Compare(toSemver)
		if cmp < 0 {
			return fmt.Sprintf("Upgraded from %s to %s.", from, to)
		}
		if cmp > 0 {
			return fmt.Sprintf("Downgraded from %s to %s.", from, to)
		}
	}

	return fmt.Sprintf("Updated from %s to %s.", from, to)
}

// CI archives may contain either command name, independently of the local name.
func archiveBinaryPath(directory, platform, architecture string) (string, error) {
	for _, name := range cli.BinaryNames() {
		if platform == "windows" {
			name += ".exe"
		}
		path := filepath.Join(directory, "bin", name)
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return path, nil
		}
	}
	root := filepath.Join(directory, "_release")
	suffix := "_" + platform + "_" + architecture
	if platform == "windows" {
		suffix += ".exe"
	}
	// Release archives can include branch names containing slashes, e.g.
	// _release/fix/example/mw_fix/example_linux_amd64. Match the complete
	// version directory and filename, rather than guessing from a basename.
	versions := map[string]map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), suffix) {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		parts := strings.Split(filepath.ToSlash(relative), "/")
		for i, part := range parts {
			for _, name := range cli.BinaryNames() {
				if !strings.HasPrefix(part, name+"_") {
					continue
				}
				version := strings.TrimSuffix(strings.TrimPrefix(strings.Join(parts[i:], "/"), name+"_"), suffix)
				// Match Makefile's SEMVER directory transformation exactly.
				if version == "" || strings.Join(parts[:i], "/") != strings.ReplaceAll(version, "v", "") {
					continue
				}
				if versions[version] == nil {
					versions[version] = map[string]string{}
				}
				versions[version][name] = path
			}
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if len(versions) > 1 {
		return "", fmt.Errorf("archive contains multiple releases for %s/%s; select a single binary URL", platform, architecture)
	}
	for _, candidates := range versions {
		for _, name := range cli.BinaryNames() {
			if path := candidates[name]; path != "" {
				return path, nil
			}
		}
	}
	return "", fmt.Errorf("archive contains no supported executable in bin/ or _release/ for %s/%s", platform, architecture)
}
