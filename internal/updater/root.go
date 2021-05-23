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
	"log"
	"os"
	"strings"

	"github.com/blang/semver"
	"github.com/rhysd/go-github-selfupdate/selfupdate"
)

/*Check ...*/
func Check(currentVersion string, gitSummary string) {
	selfupdate.EnableLog()

	if !strings.HasPrefix(gitSummary, currentVersion) || strings.HasSuffix(gitSummary,"dirty") {
		log.Println("Can only update tag built releases")
		os.Exit(1)
	}

	log.Println("Checking version " + currentVersion)
	log.Println("git summary " + gitSummary)
	v := semver.MustParse(strings.Trim(gitSummary,"v"))

	// TODO when builds are on wm.o then allow for a "dev" or "stable" update option

	latest, err := selfupdate.UpdateSelf(v, "addshore/mwcli")
	if err != nil {
		log.Println("Binary update failed:", err)
		return
	}
	if latest.Version.Equals(v) {
		// latest version is the same as current version. It means current binary is up to date.
		log.Println("Current binary is the latest version", currentVersion)
	} else {
		log.Println("Successfully updated to version", latest.Version)
		log.Println("Release note:\n", latest.ReleaseNotes)
	}
	os.Exit(0)
}