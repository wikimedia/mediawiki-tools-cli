/*Package cmd is used for command line.

Copyright © 2020 Kosta Harlan <kosta@kostaharlan.net>

This program is free software: you can redistribute it and/or modify
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
	"github.com/manifoldco/promptui"
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"

	"gerrit.wikimedia.org/r/mediawiki/tools/cli/internal/docker"
	"gerrit.wikimedia.org/r/mediawiki/tools/cli/internal/exec"
	"gerrit.wikimedia.org/r/mediawiki/tools/cli/internal/mediawiki"
)

var dockerCmd = &cobra.Command{
	Use:   "docker",
	Short: "The MediaWiki-Docker development environment",
	RunE:  nil,
}

func mediawikiOrFatal() mediawiki.MediaWiki {
	MediaWiki, err := mediawiki.ForCurrentWorkingDirectory()
	if err != nil {
		log.Fatal("❌ Please run this command within the root of the MediaWiki core repository.")
		os.Exit(1);
	}
	return MediaWiki
}

var dockerStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the development environment",
	Run: func(cmd *cobra.Command, args []string) {
		Spinner := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		Spinner.Prefix = "Starting the development environment "
		Spinner.FinalMSG = Spinner.Prefix + "(done)\n"
		options := exec.HandlerOptions{
			Spinner:     Spinner,
			Verbosity:   Verbosity,
			HandleError: handlePortError,
		}
		MediaWiki := mediawikiOrFatal()

		exec.RunCommand(options, exec.DockerComposeCommand("up", "-d"))

		if isLinuxHost() {
			fileCreated,err := docker.EnsureDockerComposeUserOverrideExists()
			if fileCreated {
				fmt.Println("Creating docker-compose.override.yml for correct user ID and group ID mapping from host to container")
			}
			if err != nil {
				log.Fatal(err)
			}
		}

		MediaWiki.EnsureCacheDirectory()

		if docker.MediaWikiComposerDependenciesNeedInstallation(exec.HandlerOptions{Verbosity: Verbosity}) {
			fmt.Println("MediaWiki has some external dependencies that need to be installed")
			prompt := promptui.Prompt{
				IsConfirm: true,
				Label:     "Install dependencies now",
			}
			_, err := prompt.Run()
			if err == nil {
				Spinner := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
				Spinner.Prefix = "Installing Composer dependencies (this may take a few minutes) "
				Spinner.FinalMSG = Spinner.Prefix + "(done)\n"

				options := exec.HandlerOptions{
					Spinner: Spinner,
					Verbosity: Verbosity,
				}
				docker.MediaWikiComposerUpdate(options)
			}

		}

		if !MediaWiki.VectorIsPresent() {
			prompt := promptui.Prompt{
				IsConfirm: true,
				Label:     "Download and use the Vector skin",
			}
			_, err := prompt.Run()
			if err == nil {
				Spinner := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
				Spinner.Prefix = "Downloading Vector "
				Spinner.FinalMSG = Spinner.Prefix + "(done)\n"

				options := exec.HandlerOptions{
					Spinner: Spinner,
					Verbosity: Verbosity,
					HandleError: func(stderr bytes.Buffer, err error) {
						if err != nil {
							log.Fatal(err)
						}
					},
				}

				MediaWiki.GitCloneVector(options)
			}

		}

		if !MediaWiki.LocalSettingsIsPresent() {
			prompt := promptui.Prompt{
				IsConfirm: true,
				Label:     "Install MediaWiki database tables and create LocalSettings.php",
			}
			_, err := prompt.Run()
			if err == nil {
				Spinner := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
				Spinner.Prefix = "Installing "
				Spinner.FinalMSG = Spinner.Prefix + "(done)\n"
				options := exec.HandlerOptions{
					Spinner: Spinner,
					Verbosity: Verbosity,
				}
				docker.MediaWikiInstall(options)
			}
		}

		printSuccess()
	},
}

var dockerExecCmd = &cobra.Command{
	Use:   "exec [service] [command] [args]",
	Short: "Run a command in the specified container",
	Args:  cobra.MinimumNArgs(2),
	PreRun: func(cmd *cobra.Command, args []string) {
		mediawiki.CheckIfInCoreDirectory()
	},
	Run: func(cmd *cobra.Command, args []string) {
		options := exec.HandlerOptions{
			Verbosity: Verbosity,
		}

		if Detach {
			args = append([]string{"-d"}, args...)
		}

		if Privileged {
			args = append([]string{"--privileged"}, args...)
		}

		if User != "" {
			args = append([]string{"-u", User}, args...)
		}

		if Index != "" {
			args = append([]string{fmt.Sprintf("--index=%v", Index)}, args...)
		}

		for _, keyvar := range Env {
			args = append([]string{fmt.Sprintf("-e %v", keyvar)}, args...)
		}

		if Workdir != "" {
			args = append([]string{fmt.Sprintf("-w %v", Workdir)}, args...)
		}

		if NoTTY {
			args = append([]string{"-T"}, args...)
			exec.RunCommand(options, exec.DockerComposeCommand("exec", args...))
		} else {
			exec.RunTTYCommand(options, exec.DockerComposeCommand("exec", args...))
		}

	},
}

var dockerDestroyCmd = &cobra.Command{
	Use:   "destroy [service...]",
	Short: "destroys the development environment or specified containers",
	Run: func(cmd *cobra.Command, args []string) {
		MediaWiki := mediawikiOrFatal()

		options := exec.HandlerOptions{
			Verbosity: Verbosity,
		}

		runArgs := append([]string{"-sfv"}, args...)
		exec.RunTTYCommand(options, exec.DockerComposeCommand("rm", runArgs...))

		if len(args) == 0 || contains(args, "mediawiki") {
			MediaWiki.RenameLocalSettings()
			MediaWiki.DeleteCache()
			MediaWiki.DeleteVendor()
		}
	},
}

func contains(slice []string, s string) bool {
	for _, i := range slice {
		if s == i {
			return true
		}
	}
	return false
}

var dockerStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop development environment",
	PreRun: func(cmd *cobra.Command, args []string) {
		mediawiki.CheckIfInCoreDirectory()
	},
	Run: func(cmd *cobra.Command, args []string) {
		Spinner := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		Spinner.Prefix = "Stopping development environment "
		Spinner.FinalMSG = Spinner.Prefix + "(done)\n"
		options := exec.HandlerOptions{
			Spinner: Spinner,
			Verbosity: Verbosity,
		}
		exec.RunCommand(options, exec.DockerComposeCommand("stop"))
	},
}

var dockerStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "List development environment status",
	PreRun: func(cmd *cobra.Command, args []string) {
		mediawiki.CheckIfInCoreDirectory()
	},
	Run: func(cmd *cobra.Command, args []string) {
		options := exec.HandlerOptions{
			Verbosity: Verbosity,
		}
		exec.RunCommand(options, exec.DockerComposeCommand("ps"))
	},
}

func printSuccess() {
	options := exec.HandlerOptions{
		Verbosity: Verbosity,
		HandleStdout: func(stdout bytes.Buffer) {
			// Replace 0.0.0.0 in the output with localhost
			fmt.Printf("Success! View MediaWiki-Docker at http://%s",
				strings.Replace(stdout.String(), "0.0.0.0", "localhost", 1))
		},
	}
	exec.RunCommand(options, exec.DockerComposeCommand("port", "mediawiki", "8080"))

}

func handlePortError(stderr bytes.Buffer, err error) {
	stdoutStderr := stderr.Bytes()
	portError := strings.Index(string(stdoutStderr), " failed: port is already allocated")
	if portError > 0 {
		// TODO: This assumes a port that is four characters long.
		log.Fatalf("Port %s is already allocated! \n\nPlease override the port via MW_DOCKER_PORT in the .env file\nYou can use the 'docker env' command to do this\nSee `mw docker env --help` for more information.",
			string(stdoutStderr[portError-4:])[0:4])
	} else if err != nil && stderr.String() != "" {
		fmt.Printf("\n%s\n%s\n", "STDERR:", stderr.String())
	}
}

func isLinuxHost() bool {
	unameCommand := exec.Command("uname")
	stdout, err := unameCommand.CombinedOutput()
	if err != nil {
		log.Fatal(err)
	}
	return string(stdout) == "Linux\n"
}

func init() {
	dockerCmd.PersistentFlags().IntVarP(&Verbosity, "verbosity", "v", 1, "verbosity level (1-2)")

	rootCmd.AddCommand(dockerCmd)

	dockerCmd.AddCommand(dockerStartCmd)
	dockerCmd.AddCommand(dockerStopCmd)
	dockerCmd.AddCommand(dockerStatusCmd)
	dockerCmd.AddCommand(dockerDestroyCmd)

	dockerExecCmd.Flags().BoolVarP(&Detach, "detach", "d", false, "Detached mode: Run command in the background.")
	dockerExecCmd.Flags().BoolVarP(&Privileged, "privileged", "p", false, "Give extended privileges to the process.")
	dockerExecCmd.Flags().StringVarP(&User, "user", "u", "", "Run the command as this user.")
	dockerExecCmd.Flags().BoolVarP(&NoTTY, "TTY", "T", false, "Disable pseudo-tty allocation. By default a TTY is allocated")
	dockerExecCmd.Flags().StringVarP(&Index, "index", "i", "", "Index of the container if there are multiple instances of a service [default: 1]")
	dockerExecCmd.Flags().StringSliceVarP(&Env, "env", "e", []string{}, "Set environment variables. Can be used multiple times")
	dockerExecCmd.Flags().StringVarP(&Workdir, "workdir", "w", "", "Path to workdir directory for this command.")
	dockerCmd.AddCommand(dockerExecCmd)
}
