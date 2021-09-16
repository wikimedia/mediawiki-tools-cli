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
	"io/ioutil"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/blang/semver"
	"github.com/rhysd/go-github-selfupdate/selfupdate"
)

/*CanUpdateFromWikimedia ...*/
func CanUpdateFromWikimedia(currentVersion string, gitSummary string, verboseOutput bool) (bool, string) {
	if verboseOutput {
		selfupdate.EnableLog()
	}

	latestRelease := latestWikimediaRelease()

	if latestRelease == "404" {
		return false, "No Wikimedia releases yet"
	}

	newVersion, newErr := semver.Parse(strings.Trim(latestRelease, "v"))
	currentVerion, currentErr := semver.Parse(strings.Trim(gitSummary, "v"))

	if newErr != nil {
		return false, "Could not remote release version?"
	}
	if currentErr != nil {
		return false, "Could not parse current git summary version '" + gitSummary + "', maybe you are not using a real release? Next release would be " + newVersion.String()
	}

	return currentVerion.Compare(newVersion) == -1, newVersion.String()
}

func latestWikimediaRelease() string {
	url := "https://releases.wikimedia.org/mwcli/latest.txt"

	client := http.Client{}

	resp, err := client.Get(url)
	if err != nil {
		panic("Something went wrong retrieving " + url)
	}
	defer resp.Body.Close()

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic("Something went wrong reading " + url)
	}
	latestContent := strings.TrimSpace(string(content))

	if strings.Contains(latestContent, "404") {
		return "404"
	}

	return latestContent
}

/*UpdateFromWikimedia ...*/
func UpdateFromWikimedia(currentVersion string, gitSummary string, verboseOutput bool) (success bool, message string) {
	if verboseOutput {
		selfupdate.EnableLog()
	}

	canUpdate, newVersionOrMessage := CanUpdateFromWikimedia(currentVersion, gitSummary, verboseOutput)
	if !canUpdate {
		return false, "No update found: " + newVersionOrMessage
	}

	assetURL := "https://releases.wikimedia.org/mwcli/" + newVersionOrMessage + "/mw_v" + newVersionOrMessage + "_" + runtime.GOOS + "_" + runtime.GOARCH

	cmdPath, err := os.Executable()
	if err != nil {
		return false, "Failed to grab local executable location"
	}

	err = selfupdate.UpdateTo(assetURL, cmdPath)
	if err != nil {
		return false, "Binary update failed" + err.Error()
	}

	return true, "successfuly updated to version " + newVersionOrMessage
}
