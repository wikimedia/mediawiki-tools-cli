/*Package eventlogging talking to Wikimedia EventLogging

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
package eventlogging

import (
	"strings"

	"github.com/spf13/cobra"
)

var recordCommands = []string{
	"codesearch",
}

func RecordCommand(cmd *cobra.Command, args []string) {

	var logString = cmd.CommandPath() + " " + strings.Join(args, " ")
	// fmt.Println(logString)
	// fmt.Println(cmd.Flags())
	// os.Exit(1)
	// Do nothing for now
	// TODO record locally on disk, and then periodically send?
}
