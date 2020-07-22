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
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"

	"gerrit.wikimedia.org/r/mediawiki/tools/cli/exec"
)

var dockerCmd = &cobra.Command{
	Use:   "docker",
	Short: "Provides subcommands for interacting with MediaWiki's Docker development environment",
	RunE:  nil,
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the development environment",
	Run: func(cmd *cobra.Command, args []string) {
		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Prefix = "Starting the development environment "
		s.Start()
		command := exec.DockerCompose("up", "-d")
		stdoutStderr, err := command.CombinedOutput()
		s.Stop()
		// If there is a port error, that's the only thing we want to output now.
		handlePortError(stdoutStderr)
		fmt.Print(string(stdoutStderr))
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

func promptToInstallMediaWiki() {
	prompt := promptui.Prompt{
		IsConfirm: true,
		Label:     "Install MediaWiki database tables and create LocalSettings.php",
	}
	_, err := prompt.Run()
	if err == nil {
		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Prefix = "Installing "
		s.Start()

		command := exec.DockerCompose(
			"exec",
			"-T",
			"mediawiki",
			"/bin/bash",
			"/docker/install.sh")
		stdoutStderr, err := command.CombinedOutput()
		fmt.Printf("%s\n", stdoutStderr)
		if err != nil {
			log.Fatal(err)
		}
		s.Stop()
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
		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Prefix = "Downloading Vector "
		s.Start()
		command := exec.Command(
			"git",
			"clone",
			"https://gerrit.wikimedia.org/r/mediawiki/skins/Vector",
			"skins/Vector")
		stdoutStderr, err := command.CombinedOutput()
		fmt.Printf("%s\n", stdoutStderr)
		if err != nil {
			log.Fatal(err)
		}
		s.Stop()
	}
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop development environment",
	PreRun: func(cmd *cobra.Command, args []string) {
		checkIfInCoreDirectory()
	},
	Run: func(cmd *cobra.Command, args []string) {
		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Prefix = "Stopping development environment "
		s.Start()
		command := exec.DockerCompose("stop")
		stdoutStderr, err := command.CombinedOutput()
		s.Stop()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s\n", stdoutStderr)
	},
}

func printSuccess() {
	portCommand := exec.DockerCompose("port", "mediawiki", "8080")
	portCommandOutput, _ := portCommand.CombinedOutput()
	// Replace 0.0.0.0 in the output with localhost
	fmt.Printf("Success! View MediaWiki-Docker at http://%s",
		strings.Replace(string(portCommandOutput), "0.0.0.0", "localhost", 1))
}

func handlePortError(stdoutStderr []byte) {
	portError := strings.Index(string(stdoutStderr), " failed: port is already allocated")
	if portError > 0 {
		// TODO: This assumes a port that is four characters long.
		log.Fatalf("Port %s is already allocated! \n\nPlease override the port via MW_DOCKER_PORT in the .env file\nYou can use the 'docker env' command to do this\nSee `mw docker env --help` for more information.",
			string(stdoutStderr[portError-4:])[0:4])
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
		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Prefix = "Installing Composer dependencies (this may take a few minutes) "
		s.Start()
		err := os.Mkdir("cache", 0700)
		if err != nil {
			log.Fatal(err)
		}
		depsCommand := exec.DockerCompose(
			"exec",
			"-T",
			"mediawiki",
			"composer",
			"update",
		)
		out, err := depsCommand.CombinedOutput()
		if err != nil {
			fmt.Print(string(out))
			log.Fatal(err)
		}
		s.Stop()
	}
}

func composerDependenciesNeedInstallation() bool {
	// Detect if composer dependencies are not installed and prompt user to install
	dependenciesCheck := exec.DockerCompose(
		"exec",
		"-T",
		"mediawiki",
		"php",
		"maintenance/install.php",
		"--help",
	)
	stdoutStderr, _ := dependenciesCheck.CombinedOutput()
	return strings.Index(string(stdoutStderr), " dependencies that need to be installed") > 0
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
	rootCmd.AddCommand(dockerCmd)

	dockerCmd.AddCommand(startCmd)
	dockerCmd.AddCommand(stopCmd)
}
