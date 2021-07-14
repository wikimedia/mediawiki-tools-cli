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
	"gerrit.wikimedia.org/r/mediawiki/tools/cli/internal/cmd"
)

var dockerEnvCmd = cmd.Env("Provides subcommands for interacting with development environment variables");
var dockerEnvDeleteCmd = cmd.EnvDelete(func()string{return mediawikiOrFatal().Directory()});
var dockerEnvSetCmd = cmd.EnvSet(func()string{return mediawikiOrFatal().Directory()});
var dockerEnvGetCmd = cmd.EnvGet(func()string{return mediawikiOrFatal().Directory()});
var dockerEnvListCmd = cmd.EnvList(func()string{return mediawikiOrFatal().Directory()});
var dockerEnvWhereCmd = cmd.EnvWhere(func()string{return mediawikiOrFatal().Directory()});

func init() {
	dockerCmd.AddCommand(dockerEnvCmd)

	dockerEnvCmd.AddCommand(dockerEnvWhereCmd)
	dockerEnvCmd.AddCommand(dockerEnvSetCmd)
	dockerEnvCmd.AddCommand(dockerEnvGetCmd)
	dockerEnvCmd.AddCommand(dockerEnvListCmd)
	dockerEnvCmd.AddCommand(dockerEnvDeleteCmd)
}
