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
	"os"
	"path"

	"gitlab.wikimedia.org/releng/cli/internal/util/paths"
)

func UpdatePermissionCheck() (bool, error) {
	// All of this is a half hack around the fact the internal library doesn't check permissison
	// https://github.com/inconshreveable/go-update/issues/46
	exPath, err := os.Executable()
	if err != nil {
		return false, err
	}

	exDir, _ := path.Split(exPath)

	// selfupdate.UpdateTo makes this file internally
	// tmpPath := exDir + "." + exFile + ".old"

	// Note, we would want to check the executabler, but not easily possible?
	// https://phabricator.wikimedia.org/T293963#7449454
	// So just check the dir for now?
	dirWrite, dirWriteErr := paths.IsWritableDir(exDir)
	if !dirWrite {
		return false, dirWriteErr
	}
	return true, nil

}

/*CanUpdate will check for updates.*/
func CanUpdate(currentVersion string, gitSummary string, verboseOutput bool) (bool, string) {
	canUpdate, release := CanUpdateFromGitlab(currentVersion, gitSummary, verboseOutput)
	if canUpdate {
		return canUpdate, release
	}

	message := "No update available"

	if verboseOutput {
		message = message + "\nCurrent version is: " + currentVersion + "\nLatest available is: " + release
	}

	// When canUpdate is false, we dont have a release to get the version string of
	return canUpdate, message
}

/*Update perform the latest update.*/
func Update(currentVersion string, gitSummary string, verboseOutput bool) (bool, string) {
	return UpdateFromGitlab(currentVersion, gitSummary, verboseOutput)
}

func CanMoveToVersion(targetVersion string) bool {
	return CanMoveToVersionFromGitlab(targetVersion)
}

func MoveToVersion(targetVersion string) (success bool, message string) {
	return MoveToVersionFromGitlab(targetVersion)
}
