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

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/exec"
	"gitlab.wikimedia.org/releng/cli/internal/mwdd"
	"gitlab.wikimedia.org/releng/cli/internal/util/ports"
)

var mwddCmd = &cobra.Command{
	Use:   "docker",
	Short: "The MediaWiki-Docker-Dev like development environment",
	RunE:  nil,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		mwdd := mwdd.DefaultForUser()
		mwdd.EnsureReady()
		if mwdd.Env().Missing("PORT") {
			if !globalOpts.NoInteraction {
				port := ""
				prompt := &survey.Input{
					Message: "What port would you like to use for your development environment?",
					Default: ports.FreeUpFrom("8080"),
				}
				err := survey.AskOne(prompt, &port)
				if err != nil {
					fmt.Println(err)
					os.Exit(1)
				}
				validityChck := ports.IsValidAndFree(port)
				if validityChck != nil {
					fmt.Println(validityChck)
					os.Exit(1)
				}

				mwdd.Env().Set("PORT", port)
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
			Verbosity: globalOpts.Verbosity,
		}
		mwdd.DefaultForUser().DownWithVolumesAndOrphans(options)
	},
}

var mwddSuspendCmd = &cobra.Command{
	Use:   "suspend",
	Short: "Suspend the Default containers",
	Run: func(cmd *cobra.Command, args []string) {
		options := exec.HandlerOptions{
			Verbosity: globalOpts.Verbosity,
		}
		mwdd.DefaultForUser().Stop([]string{}, options)
	},
}

var mwddResumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume the Default containers",
	Run: func(cmd *cobra.Command, args []string) {
		options := exec.HandlerOptions{
			Verbosity: globalOpts.Verbosity,
		}
		fmt.Println("Any services that you have not already created will show as 'failed'")
		mwdd.DefaultForUser().Start([]string{}, options)
	},
}

func init() {
	mwddCmd.AddCommand(mwddWhereCmd)
	mwddCmd.AddCommand(mwddDestroyCmd)
	mwddCmd.AddCommand(mwddSuspendCmd)
	mwddCmd.AddCommand(mwddResumeCmd)

	rootCmd.AddCommand(mwddCmd)
}
