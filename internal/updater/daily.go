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
	"log"
	"os"
	"os/user"
	"path/filepath"
	"time"
)

/*CanUpdateDaily will check for updates at most once a day*/
func CanUpdateDaily(currentVersion string, gitSummary string, verboseOutput bool) (bool, string) {
	now := time.Now().UTC()
	if now.Sub(lastCheckedTime()).Hours() < 24 {
		if verboseOutput {
			log.Println("Already checked for updates in the last 24 hours")
		}
		return false, ""
	}
	setCheckedTime(now)

	return CanUpdate(currentVersion, gitSummary, verboseOutput)
}

func lastCheckedTime() time.Time {
	if _, err := os.Stat(lastUpdateFilePath()); os.IsNotExist(err) {
		return time.Now().UTC().Add(-24 * time.Hour * 7)
	}

	content, err := ioutil.ReadFile(lastUpdateFilePath())
	if err != nil {
		log.Fatal(err)
	}
	t, err := time.Parse(time.RFC3339, string(content))
	if err != nil {
		log.Fatal(err)
	}
	return t
}

func setCheckedTime(toSet time.Time) {
	ensureDirectoryForFileOnDisk(lastUpdateFilePath())
	err := ioutil.WriteFile(lastUpdateFilePath(), []byte(toSet.Format(time.RFC3339)), 0700)
	if err != nil {
		log.Fatal(err)
	}
}

func lastUpdateFilePath() string {
	currentUser, _ := user.Current()
	return currentUser.HomeDir + string(os.PathSeparator) + ".mwcli/.lastUpdateCheck"
}

func ensureDirectoryForFileOnDisk(file string) {
	// TODO factor this method out (also used in mwdd.files)
	ensureDirectoryOnDisk(filepath.Dir(file))
}

func ensureDirectoryOnDisk(dirPath string) {
	// TODO factor this method out (also used in mwdd.files)
	if _, err := os.Stat(dirPath); err != nil {
		os.MkdirAll(dirPath, 0755)
	}
}
