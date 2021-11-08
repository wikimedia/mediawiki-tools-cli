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
	"gitlab.wikimedia.org/releng/cli/internal/cmd"
	"gitlab.wikimedia.org/releng/cli/internal/mwdd"
)

func mwddEnvDirectory() string {
	return mwdd.DefaultForUser().Directory()
}

var (
	mwddEnvCmd       = cmd.Env("Interact with the environment variables", rootCmd)
	mwddEnvDeleteCmd = cmd.EnvDelete(mwddEnvDirectory)
	mwddEnvSetCmd    = cmd.EnvSet(mwddEnvDirectory)
	mwddEnvGetCmd    = cmd.EnvGet(mwddEnvDirectory)
	mwddEnvListCmd   = cmd.EnvList(mwddEnvDirectory)
	mwddEnvWhereCmd  = cmd.EnvWhere(mwddEnvDirectory)
	mwddEnvClearCmd  = cmd.EnvClear(mwddEnvDirectory)
)

func init() {
	mwddCmd.AddCommand(mwddEnvCmd)

	mwddEnvCmd.AddCommand(mwddEnvWhereCmd)
	mwddEnvCmd.AddCommand(mwddEnvSetCmd)
	mwddEnvCmd.AddCommand(mwddEnvGetCmd)
	mwddEnvCmd.AddCommand(mwddEnvListCmd)
	mwddEnvCmd.AddCommand(mwddEnvDeleteCmd)
	mwddEnvCmd.AddCommand(mwddEnvClearCmd)
}
