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
	"strings"

	"github.com/blang/semver"
	"github.com/rhysd/go-github-selfupdate/selfupdate"
	"gitlab.wikimedia.org/releng/cli/internal/gitlab"
)

/*CanUpdateFromGitlab ...*/
func CanUpdateFromGitlab(version string, gitSummary string, verboseOutput bool) (bool, string) {
	if verboseOutput {
		selfupdate.EnableLog()
	}

	latestRelease, latestErr := gitlab.RelengCliLatestRelease()
	if latestErr != nil {
		return false, "Could not fetch latest release version from Gitlab"
	}

	newVersion, newErr := semver.Parse(strings.Trim(latestRelease.TagName, "v"))
	currentVersion, currentErr := semver.Parse(strings.Trim(version, "v"))

	if newErr != nil {
		return false, "Could not parse latest release version from Gitlab"
	}
	if currentErr != nil {
		return false, "Could not parse current version '" + version + "'. Next release would be " + newVersion.String()
	}

	return currentVersion.Compare(newVersion) == -1, newVersion.String()
}

/*UpdateFromGitlab ...*/
func UpdateFromGitlab(currentVersion string, gitSummary string, verboseOutput bool) (success bool, message string) {
	if verboseOutput {
		selfupdate.EnableLog()
	}

	canUpdate, newVersionOrMessage := CanUpdateFromGitlab(currentVersion, gitSummary, verboseOutput)
	if !canUpdate {
		return false, "No update found: " + newVersionOrMessage
	}

	// TODO refactor to avoid 2 API calls
	release, err := gitlab.RelengCliLatestRelease()
	if err != nil {
		panic(err)
	}
	link, err := gitlab.RelengCliLatestReleaseBinary()
	if err != nil {
		panic(err)
	}

	cmdPath, err := os.Executable()
	if err != nil {
		return false, "Failed to grab local executable location"
	}

	err = selfupdate.UpdateTo(link.DirectAssetURL, cmdPath)
	if err != nil {
		return false, "Binary update failed" + err.Error()
	}

	return true, "Successfully updated to version " + release.TagName + "\n\n" + release.Description
}

func CanMoveToVersionFromGitlab(targetVersion string) bool {
	_, err := gitlab.RelengCliReleaseBinary(targetVersion)
	return err == nil
}

func MoveToVersionFromGitlab(targerVersion string) (success bool, message string) {
	// TODO refactor to avoid 2 API calls
	release, err := gitlab.RelengCliRelease(targerVersion)
	if err != nil {
		panic(err)
	}
	link, err := gitlab.RelengCliReleaseBinary(targerVersion)
	if err != nil {
		panic(err)
	}

	cmdPath, err := os.Executable()
	if err != nil {
		return false, "Failed to grab local executable location"
	}

	err = selfupdate.UpdateTo(link.DirectAssetURL, cmdPath)
	if err != nil {
		return false, "Binary update failed" + err.Error()
	}

	return true, "Successfully updated to version " + release.TagName + "\n\n" + release.Description
}
