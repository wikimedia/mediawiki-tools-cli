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
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"

	"gerrit.wikimedia.org/r/mediawiki/tools/cli/internal/exec"
)

// Verbose mode.
var Verbosity int
var Detach bool
var Privileged bool
var User string
var NoTTY bool
var Index string
var Env []string
var Workdir string

var dockerCmd = &cobra.Command{
	Use:   "docker",
	Short: "Provides subcommands for interacting with MediaWiki's Docker development environment",
	RunE:  nil,
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the development environment",
	Run: func(cmd *cobra.Command, args []string) {
		Spinner := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		Spinner.Prefix = "Starting the development environment "
		Spinner.FinalMSG = Spinner.Prefix + "(done)\n"
		options := exec.HandlerOptions{
			Verbosity:   Verbosity,
			HandleError: handlePortError,
		}
		exec.RunCommand(options, exec.DockerComposeCommand("up", "-d"), Spinner)

		err := os.MkdirAll("cache", 0700)
		if err != nil {
			log.Fatal(err)
		}

		if composerDependenciesNeedInstallation() {
			promptToInstallComposerDependencies()
		}

		if !vectorIsPresent() {
			promptToCloneVector()
		}

		if !localSettingsIsPresent() {
			promptToInstallMediaWiki()
		}

		printSuccess()
	},
	PreRun: func(cmd *cobra.Command, args []string) {
		checkIfInCoreDirectory()
		if isLinuxHost() {
			// TODO: We should also check the contents for correctness, maybe
			// using docker-compose config and asserting that UID/GID mapping is present
			// and with correct values.
			_, err := os.Stat("docker-compose.override.yml")
			if err != nil {
				fmt.Println("Creating docker-compose.override.yml for correct user ID and group ID mapping from host to container")
				var data = `
version: '3.7'
services:
  mediawiki:
    user: "${MW_DOCKER_UID}:${MW_DOCKER_GID}"
`
				file, err := os.Create("docker-compose.override.yml")
				if err != nil {
					log.Fatal(err)
				}
				defer file.Close()
				_, err = file.WriteString(data)
				if err != nil {
					log.Fatal(err)
				}
				file.Sync()
			}
		}
	},
}

var execCmd = &cobra.Command{
	Use:   "exec [service] [command] [args]",
	Short: "Run a command in the specified container",
	Args:  cobra.MinimumNArgs(2),
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
			exec.RunCommand(options, exec.DockerComposeCommand("exec", args...), nil)
		} else {
			exec.RunTTYCommand(options, exec.DockerComposeCommand("exec", args...))
		}

	},
}

func promptToInstallMediaWiki() {
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
			Verbosity: Verbosity,
		}
		exec.RunCommand(
			options,
			exec.DockerComposeCommand(
				"exec",
				"-T",
				"mediawiki",
				"/bin/bash",
				"/docker/install.sh"),
			Spinner)
	}
}

func localSettingsIsPresent() bool {
	info, err := os.Stat("LocalSettings.php")
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func vectorIsPresent() bool {
	info, err := os.Stat("skins/Vector")
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

func promptToCloneVector() {
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
			Verbosity: Verbosity,
			HandleError: func(stderr bytes.Buffer, err error) {
				if err != nil {
					log.Fatal(err)
				}
			},
		}

		exec.RunCommand(options, exec.Command(
			"git",
			"clone",
			"https://gerrit.wikimedia.org/r/mediawiki/skins/Vector",
			"skins/Vector"), Spinner)
	}
}

var destroyCmd = &cobra.Command{
	Use:   "destroy [service...]",
	Short: "destroys the development environment or specified containers",
	PreRun: func(cmd *cobra.Command, args []string) {
		checkIfInCoreDirectory()
	},
	Run: func(cmd *cobra.Command, args []string) {
		options := exec.HandlerOptions{
			Verbosity: Verbosity,
		}

		runArgs := append([]string{"-sfv"}, args...)
		exec.RunTTYCommand(options, exec.DockerComposeCommand("rm", runArgs...))

		if len(args) == 0 || contains(args, "mediawiki") {
			renameLocalSettings()
			deleteCoreCache()
			deleteCoreVendor()
		}
	},
}

func renameLocalSettings() {
	const layout = "2006-01-02T15:04:05-0700"

	Spinner := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	Spinner.Prefix = "Renaming LocalSettings file "
	Spinner.FinalMSG = Spinner.Prefix + "(done)\n"

	Spinner.Start()
	t := time.Now()
	localSettingsName := "LocalSettings-" + t.Format(layout) + ".php"

	err := os.Rename("LocalSettings.php", localSettingsName)

	if err != nil {
		log.Fatal(err)
	}

	Spinner.Stop()
}

func deleteCoreCache() {
	Spinner := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	Spinner.Prefix = "Deleting cache files "
	Spinner.FinalMSG = Spinner.Prefix + "(done)\n"

	Spinner.Start()

	err := os.Rename("cache/.htaccess", ".htaccess")
	if err != nil {
		log.Fatal(err)
	}

	err = os.RemoveAll("./cache/")
	if err != nil {
		log.Fatal(err)
	}

	err = os.Mkdir("cache", 0700)
	if err != nil {
		log.Fatal(err)
	}

	err = os.Rename(".htaccess", "cache/.htaccess")
	if err != nil {
		log.Fatal(err)
	}

	Spinner.Stop()
}

func deleteCoreVendor() {
	Spinner := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	Spinner.Prefix = "Deleting vendor files "
	Spinner.FinalMSG = Spinner.Prefix + "(done)\n"

	Spinner.Start()

	err := os.RemoveAll("./vendor")
	if err != nil {
		log.Fatal(err)
	}

	err = os.Mkdir("vendor", 0700)
	if err != nil {
		log.Fatal(err)
	}

	Spinner.Stop()
}

func contains(slice []string, s string) bool {
	for _, i := range slice {
		if s == i {
			return true
		}
	}
	return false
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop development environment",
	PreRun: func(cmd *cobra.Command, args []string) {
		checkIfInCoreDirectory()
	},
	Run: func(cmd *cobra.Command, args []string) {
		Spinner := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		Spinner.Prefix = "Stopping development environment "
		Spinner.FinalMSG = Spinner.Prefix + "(done)\n"
		options := exec.HandlerOptions{
			Verbosity: Verbosity,
		}
		exec.RunCommand(options, exec.DockerComposeCommand("stop"), Spinner)
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "List development environment status",
	PreRun: func(cmd *cobra.Command, args []string) {
		checkIfInCoreDirectory()
	},
	Run: func(cmd *cobra.Command, args []string) {
		options := exec.HandlerOptions{
			Verbosity: Verbosity,
		}
		exec.RunCommand(options, exec.DockerComposeCommand("ps"), nil)
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
	exec.RunCommand(options, exec.DockerComposeCommand("port", "mediawiki", "8080"), nil)

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

func promptToInstallComposerDependencies() {
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
			Verbosity: Verbosity,
		}
		exec.RunCommand(options,
			exec.DockerComposeCommand(
				"exec",
				"-T",
				"mediawiki",
				"composer",
				"update",
			),
			Spinner)
	}
}

func composerDependenciesNeedInstallation() bool {
	// Detect if composer dependencies are not installed and prompt user to install
	options := exec.HandlerOptions{
		Verbosity: Verbosity,
	}
	err := exec.RunCommand(options,
		exec.DockerComposeCommand(
			"exec",
			"-T",
			"mediawiki",
			"php",
			"-r",
			"require_once dirname( __FILE__ ) . '/includes/PHPVersionCheck.php'; $phpVersionCheck = new PHPVersionCheck(); $phpVersionCheck->checkVendorExistence();",
		),
		nil)
	return err != nil
}

func checkIfInCoreDirectory() {
	b, err := ioutil.ReadFile(".gitreview")
	if err != nil || !strings.Contains(string(b), "project=mediawiki/core.git") {
		log.Fatal("❌ Please run this command within the root of the MediaWiki core repository.")
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

	dockerCmd.AddCommand(startCmd)
	dockerCmd.AddCommand(stopCmd)
	dockerCmd.AddCommand(statusCmd)
	dockerCmd.AddCommand(destroyCmd)

	execCmd.Flags().BoolVarP(&Detach, "detach", "d", false, "Detached mode: Run command in the background.")
	execCmd.Flags().BoolVarP(&Privileged, "privileged", "p", false, "Give extended privileges to the process.")
	execCmd.Flags().StringVarP(&User, "user", "u", "", "Run the command as this user.")
	execCmd.Flags().BoolVarP(&NoTTY, "TTY", "T", false, "Disable pseudo-tty allocation. By default a TTY is allocated")
	execCmd.Flags().StringVarP(&Index, "index", "i", "", "Index of the container if there are multiple instances of a service [default: 1]")
	execCmd.Flags().StringSliceVarP(&Env, "env", "e", []string{}, "Set environment variables. Can be used multiple times")
	execCmd.Flags().StringVarP(&Workdir, "workdir", "w", "", "Path to workdir directory for this command.")
	dockerCmd.AddCommand(execCmd)
}
