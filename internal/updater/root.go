/*Package updater is used to update the cli

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
package updater

import (
	"gerrit.wikimedia.org/r/mediawiki/tools/cli/internal/config"
)

/*CanUpdate will check for updates*/
func CanUpdate(currentVersion string, gitSummary string, verboseOutput bool) (bool, string) {
	c := config.LoadFromDisk()
	if c.UpdateChannel == config.UpdateChannelDev {
		canUpdate, release := CanUpdateFromAddshore(currentVersion, gitSummary, verboseOutput)
		if canUpdate {
			return canUpdate, release.Version.String()
		}
		// When canUpdate is false, we dont have a release to get the version string of
		return canUpdate, "Can't currently update"
	}
	if c.UpdateChannel == config.UpdateChannelStable {
		return CanUpdateFromWikimedia(currentVersion, gitSummary, verboseOutput)
	}
	panic("Unexpected update channel")
}

/*Update perform the latest update*/
func Update(currentVersion string, gitSummary string, verboseOutput bool) (bool, string) {
	c := config.LoadFromDisk()
	if c.UpdateChannel == config.UpdateChannelDev {
		return UpdateFromAddshore(currentVersion, gitSummary, verboseOutput)
	}
	if c.UpdateChannel == config.UpdateChannelStable {
		// TODO implement me
		panic("Not yet implemented")
	}
	panic("Unexpected update channel")
}
