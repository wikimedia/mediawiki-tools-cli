/*Package cmd is used for command line.

Copyright © 2020 Addshore

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
	"errors"
	"fmt"
	"os"
	"strconv"

	"gerrit.wikimedia.org/r/mediawiki/tools/cli/internal/config"
	"gerrit.wikimedia.org/r/mediawiki/tools/cli/internal/mwdd"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var devCmd = &cobra.Command{
	Use:   "dev",
	Aliases: []string{"mwdd","docker"},
	Short: "Interact with development environments",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		fmt.Println(11111);
		c := config.LoadFromDisk()
		if(c.DevMode != config.ConfigDevModeMwdd && c.DevMode != config.ConfigDevModeDocker){
			wizardDevMode()
		}

		c = config.LoadFromDisk()

		if(c.DevMode == config.ConfigDevModeMwdd) {
			mwdd := mwdd.DefaultForUser()
			mwdd.EnsureReady()
			if(mwdd.Env().Missing("PORT")){
				prompt := promptui.Prompt{
					Label:     "What port would you like to use for your development environment?",
					// TODO suggest a port that is definitely available for listening on
					Default: "8080",
					Validate: func(input string) error {
						// TODO check the port can be listened on?
						// https://coolaj86.com/articles/how-to-test-if-a-port-is-available-in-go/
						_, err := strconv.ParseFloat(input, 64)
						if err != nil {
							return errors.New("Invalid number")
						}
						return nil
					},
				}
				value, err := prompt.Run()
				if err == nil {
					mwdd.Env().Set("PORT",value)
				} else {
					fmt.Println("Can't continue without a port")
					os.Exit(1)
				}
			}
		}
	},
}

func wizardDevMode() {
	c := config.LoadFromDisk()
	fmt.Println("You need to choose a development environment mode in order to continue:")
	fmt.Println(" - '"+config.ConfigDevModeDocker+"' will provide basic CLI tooling around the docker-compose.json file in mediawiki.git.")
	fmt.Println(" - '"+config.ConfigDevModeMwdd+"' will provide advanced CLI tooling around a new mediawiki-docker-dev inspired development environment.")
	prompt := promptui.Prompt{
		Label: "Which would you like to use? '"+config.ConfigDevModeDocker+"' (mediawiki-docker) or '"+config.ConfigDevModeMwdd+"' (mediawiki-docker-dev)?",
		Default: config.ConfigDevModeMwdd,
		Validate: func(input string) error {
			if(input != config.ConfigDevModeMwdd && input != config.ConfigDevModeDocker) {
				return errors.New("Invalid development environment type")
			}
			return nil
		},
	}
	value, err := prompt.Run()
	if err == nil {
		c.DevMode = value
		c.WriteToDisk()
	} else {
		fmt.Println("Can't continue without a development environment mode set")
		os.Exit(1)
	}
}

func init() {
	c:= config.LoadFromDisk()
	if(c.DevMode != config.ConfigDevModeMwdd && c.DevMode != config.ConfigDevModeDocker){
		wizardDevMode()
		c= config.LoadFromDisk()
	}

	// Root dev command
	rootCmd.AddCommand(devCmd)
	devCmd.PersistentFlags().IntVarP(&Verbosity, "verbosity", "v", 1, "verbosity level (1-2)")

	// docker mode
	if(c.DevMode == config.ConfigDevModeDocker){
		dockerStartCmd.Flags().BoolVarP(&NonInteractive, "acceptPrompts", "y", false, "Answer yes to all prompts")

		dockerExecCmd.Flags().BoolVarP(&Detach, "detach", "d", false, "Detached mode: Run command in the background.")
		dockerExecCmd.Flags().BoolVarP(&Privileged, "privileged", "p", false, "Give extended privileges to the process.")
		dockerExecCmd.Flags().StringVarP(&User, "user", "u", "", "Run the command as this user.")
		dockerExecCmd.Flags().BoolVarP(&NoTTY, "TTY", "T", false, "Disable pseudo-tty allocation. By default a TTY is allocated")
		dockerExecCmd.Flags().StringVarP(&Index, "index", "i", "", "Index of the container if there are multiple instances of a service [default: 1]")
		dockerExecCmd.Flags().StringSliceVarP(&Env, "env", "e", []string{}, "Set environment variables. Can be used multiple times")
		dockerExecCmd.Flags().StringVarP(&Workdir, "workdir", "w", "", "Path to workdir directory for this command.")
	
		devCmd.AddCommand(dockerStartCmd)
		devCmd.AddCommand(dockerStopCmd)
		devCmd.AddCommand(dockerStatusCmd)
		devCmd.AddCommand(dockerDestroyCmd)
		devCmd.AddCommand(dockerExecCmd)
		devCmd.AddCommand(dockerEnvCmd)
	}

	// mwdd mode
	if(c.DevMode == config.ConfigDevModeMwdd){
		devCmd.AddCommand(mwddWhereCmd)
		devCmd.AddCommand(mwddDestroyCmd)
		devCmd.AddCommand(mwddSuspendCmd)
		devCmd.AddCommand(mwddResumeCmd)

		devCmd.AddCommand(mwddAdminerCmd)
		devCmd.AddCommand(mwddDockerComposeCmd)
		devCmd.AddCommand(mwddEnvCmd)
		devCmd.AddCommand(mwddGraphiteCmd)
		devCmd.AddCommand(mwddMediawikiCmd)
		devCmd.AddCommand(mwddMySQLReplicaCmd)
		devCmd.AddCommand(mwddMySQLCmd)
		devCmd.AddCommand(mwddPhpMyAdminCmd)
		devCmd.AddCommand(mwddPostgresCmd)
		devCmd.AddCommand(mwddRedisCmd)
	}
}
