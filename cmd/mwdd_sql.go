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
	"github.com/spf13/cobra"
)


var mwddSQLCmd = &cobra.Command{
	Use:   "sql",
	Short: "Sql service",
	RunE:  nil,
}

var mwddSQLCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create the SQL containers",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Not yet implemented!");
	},
}

var mwddSQLDestroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy the SQL containers",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Not yet implemented!");
	},
}

var mwddSQLSuspendCmd = &cobra.Command{
	Use:   "suspend",
	Short: "Suspend the SQL containers",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Not yet implemented!");
	},
}

var mwddSQLResumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume the SQL containers",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Not yet implemented!");
	},
}

func init() {
	mwddCmd.AddCommand(mwddSQLCmd)
	mwddSQLCmd.AddCommand(mwddSQLCreateCmd)
	mwddSQLCmd.AddCommand(mwddSQLDestroyCmd)
	mwddSQLCmd.AddCommand(mwddSQLSuspendCmd)
	mwddSQLCmd.AddCommand(mwddSQLResumeCmd)
}
