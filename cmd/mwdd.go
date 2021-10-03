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
	"fmt"
	"os"

	"gerrit.wikimedia.org/r/mediawiki/tools/cli/internal/exec"
	"gerrit.wikimedia.org/r/mediawiki/tools/cli/internal/mwdd"
	"gerrit.wikimedia.org/r/mediawiki/tools/cli/internal/util/ports"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var mwddCmd = &cobra.Command{
	Use:   "docker",
	Short: "The MediaWiki-Docker-Dev like development environment",
	RunE:  nil,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		mwdd := mwdd.DefaultForUser()
		mwdd.EnsureReady()
		if mwdd.Env().Missing("PORT") {
			if !NoInteraction {
				prompt := promptui.Prompt{
					Label:    "What port would you like to use for your development environment?",
					Default:  ports.FreeUpFrom("8080"),
					Validate: ports.IsValidAndFree,
				}
				value, err := prompt.Run()
				if err == nil {
					mwdd.Env().Set("PORT", value)
				} else {
					fmt.Println("Can't continue without a port")
					os.Exit(1)
				}
			} else {
				mwdd.Env().Set("PORT", ports.FreeUpFrom("8080"))
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

var mwddDestroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy the all containers",
	Run: func(cmd *cobra.Command, args []string) {
		options := exec.HandlerOptions{
			Verbosity: Verbosity,
		}
		mwdd.DefaultForUser().DownWithVolumesAndOrphans(options)
	},
}

var mwddSuspendCmd = &cobra.Command{
	Use:   "suspend",
	Short: "Suspend the Default containers",
	Run: func(cmd *cobra.Command, args []string) {
		options := exec.HandlerOptions{
			Verbosity: Verbosity,
		}
		mwdd.DefaultForUser().Stop([]string{}, options)
	},
}

var mwddResumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume the Default containers",
	Run: func(cmd *cobra.Command, args []string) {
		options := exec.HandlerOptions{
			Verbosity: Verbosity,
		}
		fmt.Println("Any services that you have not already created will show as 'failed'")
		mwdd.DefaultForUser().Start([]string{}, options)
	},
}

func init() {
	mwddCmd.PersistentFlags().IntVarP(&Verbosity, "verbosity", "v", 1, "verbosity level (1-2)")
	mwddCmd.PersistentFlags().BoolVarP(&NoInteraction, "no-interaction", "n", false, "Do not ask any interactive question")

	mwddCmd.AddCommand(mwddWhereCmd)
	mwddCmd.AddCommand(mwddDestroyCmd)
	mwddCmd.AddCommand(mwddSuspendCmd)
	mwddCmd.AddCommand(mwddResumeCmd)

	rootCmd.AddCommand(mwddCmd)
}
