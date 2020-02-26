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
	"github.com/spf13/cobra"
	"log"
	"os/exec"
	"github.com/briandowns/spinner"
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
		s.Start()
		command := exec.Command( "docker-compose", "up", "-d" )
		stdoutStderr, err := command.CombinedOutput()
		if err != nil {
			log.Fatal(err)
		}
		s.Stop()
		fmt.Printf("%s\n", stdoutStderr )
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
		if err != nil {
			log.Fatal(err)
		}
		s.Stop()
		fmt.Printf("%s\n", stdoutStderr )
	},
}

func init() {
	rootCmd.AddCommand(dockerCmd)
	dockerCmd.AddCommand( startCmd )
	dockerCmd.AddCommand( stopCmd )
}
