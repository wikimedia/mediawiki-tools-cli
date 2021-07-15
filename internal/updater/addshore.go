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
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/blang/semver"
	"github.com/rhysd/go-github-selfupdate/selfupdate"
)

/*CanUpdateFromAddshore ...*/
func CanUpdateFromAddshore(currentVersion string, gitSummary string, verboseOutput bool) (bool, *selfupdate.Release) {
	if verboseOutput {
		selfupdate.EnableLog()
	}

	// TODO when builds are on wm.o then allow for a "dev" or "stable" update option and checks

	v, err := semver.Parse(strings.Trim(gitSummary, "v"))
	if err != nil {
		if verboseOutput {
			log.Println("Could not parse git summary version, maybe you are not using a real release?")
		}
		return false, nil
	}

	rel, ok, err := selfupdate.DetectLatest("addshore/mwcli")
	if err != nil {
		if verboseOutput {
			log.Println("Some unknown error occurred")
		}
		return false, rel
	}
	if !ok {
		if verboseOutput {
			log.Println("No release detected. Current version is considered up-to-date")
		}
		return false, rel
	}
	if v.Equals(rel.Version) {
		if verboseOutput {
			log.Println("Current version", v, "is the latest. Update is not needed")
		}
		return false, rel
	}
	if verboseOutput {
		log.Println("Update available", rel.Version)
	}
	return true, rel
}

/*UpdateFromAddshoreTo ...*/
func UpdateFromAddshoreTo(release selfupdate.Release, verboseOutput bool) (success bool, message string) {
	if verboseOutput {
		selfupdate.EnableLog()
	}

	cmdPath, err := os.Executable()
	if err != nil {
		return false, "Failed to grab local executable location"
	}

	err = selfupdate.UpdateTo(release.AssetURL, cmdPath)
	if err != nil {
		return false, "Binary update failed" + err.Error()
	}

	return true, "Successfully updated to version" + release.Version.String() + "\nRelease note:\n" + release.ReleaseNotes
}

/*UpdateFromAddshore ...*/
func UpdateFromAddshore(currentVersion string, gitSummary string, verboseOutput bool) (success bool, message string) {
	canUpdate, nextRelease := CanUpdateFromAddshore(currentVersion, gitSummary, verboseOutput)
	if !canUpdate || nextRelease == nil {
		return false, "Nothing to update to"
	}

	updateSuccess, updateMessage := UpdateFromAddshoreTo(*nextRelease, verboseOutput)
	fmt.Println(updateMessage)
	if !updateSuccess {
		os.Exit(1)
	}
	return updateSuccess, updateMessage
}
