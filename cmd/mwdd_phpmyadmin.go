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
	"gerrit.wikimedia.org/r/mediawiki/tools/cli/internal/exec"
	"gerrit.wikimedia.org/r/mediawiki/tools/cli/internal/mwdd"
	"github.com/spf13/cobra"
)


var mwddPhpMyAdminCmd = &cobra.Command{
	Use:   "phpmyadmin",
	Short: "phpMyAdmin service",
	RunE:  nil,
}

var mwddPhpMyAdminCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a PhpMyAdmin container",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		mwdd.DefaultForUser().UpDetached(
			[]string{"phpmyadmin"},
			exec.HandlerOptions{
				Verbosity:   Verbosity,
			},
		)
	},
}

var mwddPhpMyAdminDestroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy the PhpMyAdmin container",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity:   Verbosity,
		}
		mwdd.DefaultForUser().Rm( []string{"phpmyadmin"},options)
		mwdd.DefaultForUser().RmVolumes( []string{"phpmyadmin-data"},options)
	},
}

var mwddPhpMyAdminSuspendCmd = &cobra.Command{
	Use:   "suspend",
	Short: "Suspend the PhpMyAdmin container",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity:   Verbosity,
		}
		mwdd.DefaultForUser().Stop( []string{"phpmyadmin"},options)
	},
}

var mwddPhpMyAdminResumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume the PhpMyAdmin container",
	Run: func(cmd *cobra.Command, args []string) {
		mwdd.DefaultForUser().EnsureReady()
		options := exec.HandlerOptions{
			Verbosity:   Verbosity,
		}
		mwdd.DefaultForUser().Start( []string{"phpmyadmin"},options)
	},
}

func init() {
	mwddCmd.AddCommand(mwddPhpMyAdminCmd)
	mwddPhpMyAdminCmd.AddCommand(mwddPhpMyAdminCreateCmd)
	mwddPhpMyAdminCmd.AddCommand(mwddPhpMyAdminDestroyCmd)
	mwddPhpMyAdminCmd.AddCommand(mwddPhpMyAdminSuspendCmd)
	mwddPhpMyAdminCmd.AddCommand(mwddPhpMyAdminResumeCmd)
}
