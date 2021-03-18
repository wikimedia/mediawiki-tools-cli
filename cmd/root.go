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
	"os"

	"github.com/spf13/cobra"
)

// Verbosity indicating verbose mode.
var Verbosity int

// These vars are currently used by the docker exec command

// Detach run docker command with -d
var Detach bool
// Privileged run docker command with --privileged
var Privileged bool
// User run docker command with the specified -u
var User string
// NoTTY run docker command with -T
var NoTTY bool
// Index run the docker command with the specified --index
var Index string
// Env run the docker command with the specified env vars
var Env []string
// Workdir run the docker command with this working directory
var Workdir string

var rootCmd = &cobra.Command{
	Use:   "mw",
	Short: "Developer utilities for working with MediaWiki",
}

/*Execute the root command*/
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
