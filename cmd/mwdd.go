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

	"gerrit.wikimedia.org/r/mediawiki/tools/cli/internal/exec"
	"gerrit.wikimedia.org/r/mediawiki/tools/cli/internal/mwdd"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var mwddCmd = &cobra.Command{
	Use:   "mwdd",
	Short: "The MediaWiki-Docker-Dev like development environment",
	RunE:  nil,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		mwdd := mwdd.DefaultForUser()
		mwdd.EnsureReady()
		if(mwdd.Env().Missing("PORT")){
			prompt := promptui.Prompt{
				Label:     "What port would you like to use for your development environment?",
				// TODO suggest a port that is defiantly availbile for listening on
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
	},
}

var mwddWhereCmd = &cobra.Command{
	Use:   "where",
	Short: "States the working directory for the environment",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(mwdd.DefaultForUser().Directory())
	},
}

var mwddCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create the Default containers",
	Run: func(cmd *cobra.Command, args []string) {
		options := exec.HandlerOptions{
			Verbosity:   Verbosity,
		}
		// TODO mediawiki should come from some default definition set?
		mwdd.DefaultForUser().UpDetached( []string{"mediawiki"}, options )
		// TODO add functionality for writing to the hosts file...
		//mwdd.DefaultForUser().EnsureHostsFile()
	},
}

var mwddDestroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy the Default containers",
	Run: func(cmd *cobra.Command, args []string) {
		options := exec.HandlerOptions{
			Verbosity:   Verbosity,
		}
		mwdd.DefaultForUser().DownWithVolumesAndOrphans( options )
	},
}

var mwddSuspendCmd = &cobra.Command{
	Use:   "suspend",
	Short: "Suspend the Default containers",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Not yet implemented!");
	},
}

var mwddResumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume the Default containers",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Not yet implemented!");
	},
}

func init() {
	mwddCmd.PersistentFlags().IntVarP(&Verbosity, "verbosity", "v", 1, "verbosity level (1-2)")

	mwddCmd.AddCommand(mwddWhereCmd)
	mwddCmd.AddCommand(mwddCreateCmd)
	mwddCmd.AddCommand(mwddDestroyCmd)
	mwddCmd.AddCommand(mwddSuspendCmd)
	mwddCmd.AddCommand(mwddResumeCmd)

	rootCmd.AddCommand(mwddCmd)
}
