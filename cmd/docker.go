/*
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
	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
	"github.com/manifoldco/promptui"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

var dockerCmd = &cobra.Command{
	Use:   "docker",
	Short: "Provides subcommands for interacting with MediaWiki's docker development environment",
	RunE: nil,
}

var startCmd = &cobra.Command{
	Use: "start",
	Short: "Start the development environment",
	Run: func(cmd *cobra.Command, args []string) {
		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Prefix = "Starting the development environment "
		s.Start()
		command := exec.Command( "docker-compose", "up", "-d" )
		if isLinuxHost() {
			command.Env = os.Environ()
			// TODO: Don't hardcode this.
			command.Env = append(command.Env, "MW_DOCKER_UID=1000", "MW_DOCKER_GID=1000")
		}
		stdoutStderr, err := command.CombinedOutput()
		fmt.Print( string( stdoutStderr ) )
		s.Stop()
		portError := strings.Index( string( stdoutStderr ), " failed: port is already allocated" )
		if portError > 0 {
			// TODO: This breaks if someone set port 80 for example.
			fmt.Println( string(stdoutStderr ))
			fmt.Printf( "Port %s is already allocated! \n\nPlease override the port via docker-compose.override.yml, see https://www.mediawiki.org/wiki/MediaWiki-Docker for instructions\n",
				string(stdoutStderr[portError-4: ] )[0:4] )
			os.Exit( 1 )
		}

		// Detect if composer dependencies are not installed and prompt user to install
		dependenciesCheck := exec.Command(
			"docker-compose",
			"exec",
			"-T",
			"mediawiki",
			"php",
			"maintenance/install.php",
			"--help",
		)
		stdoutStderr, _ = dependenciesCheck.CombinedOutput()
		if strings.Index( string( stdoutStderr ), " dependencies that need to be installed" ) > 0 {
			fmt.Println( "MediaWiki has some external dependencies that need to be installed")
			prompt := promptui.Prompt{
				IsConfirm: true,
				Label: "Install dependencies now",
			}
			_, err = prompt.Run()
			if err == nil {
				s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
				s.Prefix = "Installing Composer dependencies (this may take a few minutes) "
				s.Start()
				os.Mkdir("cache", 0700)
				depsCommand := exec.Command(
					"docker-compose",
					"exec",
					"-T",
					"mediawiki",
					"composer",
					"update",
				)
				out, err := depsCommand.CombinedOutput()
				if err != nil {
					fmt.Print(string(out))
					log.Fatal( err )
					os.Exit( 1 )
				}
				s.Stop()
			}
		}

		portCommand := exec.Command( "docker-compose", "port", "mediawiki", "8080" )
		portCommandOutput, err := portCommand.CombinedOutput()
		// Replace 0.0.0.0 with localhost
		fmt.Printf( "Success! View MediaWiki-Docker at http://%s",
			strings.Replace( string( portCommandOutput ), "0.0.0.0", "localhost", 1 ) )
	},
	PreRun: func(cmd *cobra.Command, args []string) {
		if isInCoreDirectory() == false {
			os.Exit( 1 )
		}
		if isLinuxHost() {
			// TODO: We need to also check the contents, making a lazy assumption for now.
			_, err := os.Stat("docker-compose.override.yml")
			if err != nil {
				fmt.Println( "Creating docker-compose.override.yml for correct user ID and group ID mapping from host to container")
				var data = `
version: '3.7'
services:
  mediawiki:
    user: "${MW_DOCKER_UID}:${MW_DOCKER_GID}"
`
				file, err := os.Create("docker-compose.override.yml")
				if err != nil {
					log.Fatal( err )
				}
				defer file.Close()
				_, err = file.WriteString(data)
				if err != nil {
					log.Fatal( err )
				}
				file.Sync()
			}
		}
	},
}

var stopCmd = &cobra.Command{
	Use: "stop",
	Short: "Stop development environment",
	Run: func(cmd *cobra.Command, args []string) {
		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Start()
		command := exec.Command( "docker-compose", "stop" )
		stdoutStderr, err := command.CombinedOutput()
		s.Stop()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s\n", stdoutStderr )
	},
}

func isInCoreDirectory() bool {
	if _, err := os.Stat("README.mediawiki"); err == nil {
		return true
	}

	fmt.Println("❌ Please run this command within the root of the MediaWiki core repository.")
	return false
}

func isLinuxHost() bool {
	unameCommand := exec.Command( "uname" )
	stdout, err := unameCommand.CombinedOutput()
	if err != nil {
		log.Fatal( err )
		os.Exit( 1 )
	}
	return string( stdout ) == "Linux\n"
}

func init() {
	rootCmd.AddCommand(dockerCmd)

	dockerCmd.AddCommand( startCmd )
	dockerCmd.AddCommand( stopCmd )
}
