/*Package mediawiki is used to interact with MediaWiki

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
package mediawiki

import (
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"gerrit.wikimedia.org/r/mediawiki/tools/cli/internal/exec"
)

/*MediaWiki representation of a MediaWiki install directory*/
type MediaWiki string

/*NotMediaWikiDirectory error when a directory appears to not contain MediaWiki code*/
type NotMediaWikiDirectory struct {
	directory string
}
func (e *NotMediaWikiDirectory) Error() string {
	return e.directory + " doesn't look like a MediaWiki directory"
}

/*ForDirectory returns a MediaWiki for the current working directory*/
func ForDirectory( directory string ) (MediaWiki, error) {
	return MediaWiki(directory), errorIfDirectoryDoesNotLookLikeCore(directory)
}

/*ForCurrentWorkingDirectory returns a MediaWiki for the current working directory*/
func ForCurrentWorkingDirectory() (MediaWiki, error) {
	currentWorkingDirectory, _ := os.Getwd()
	return ForDirectory(currentWorkingDirectory)
}

/*CheckIfInCoreDirectory checks that the current working directory looks like a MediaWiki directory*/
func CheckIfInCoreDirectory() {
	_, err := ForCurrentWorkingDirectory()
	if err != nil {
		log.Fatal("❌ Please run this command within the root of the MediaWiki core repository.")
	}
}

func errorIfDirectoryDoesNotLookLikeCore(directory string) error {
	b, err := ioutil.ReadFile(directory + string(os.PathSeparator) + ".gitreview")
	if err != nil || !strings.Contains(string(b), "project=mediawiki/core.git") {
		return &NotMediaWikiDirectory{directory}
	}
	return nil
}

/*Directory the directory containing MediaWiki*/
func (m MediaWiki) Directory() string {
	return string(m)
}

/*Path within the MediaWiki directory*/
func (m MediaWiki) Path(subPath string) string {
	return m.Directory() + string(os.PathSeparator) + subPath
}

/*EnsureCacheDirectory ...*/
func (m MediaWiki) EnsureCacheDirectory() {
	err := os.MkdirAll("cache", 0700)
	if err != nil {
		log.Fatal(err)
	}
}

/*VectorIsPresent ...*/
func (m MediaWiki) VectorIsPresent() bool {
	info, err := os.Stat(m.Path("skins/Vector"))
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

/*GitCloneVector ...*/
func (m MediaWiki) GitCloneVector(options exec.HandlerOptions) {
	exec.RunCommand(options, exec.Command(
		"git",
		"clone",
		"https://gerrit.wikimedia.org/r/mediawiki/skins/Vector",
		m.Path("skins/Vector")))
}

/*LocalSettingsIsPresent ...*/
func (m MediaWiki) LocalSettingsIsPresent() bool {
	info, err := os.Stat(m.Path("LocalSettings.php"))
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

/*LocalSettingsContains ...*/
func (m MediaWiki) LocalSettingsContains( text string) bool {
    b, err := ioutil.ReadFile(m.Path("LocalSettings.php"))
    if err != nil {
        panic(err)
    }
    s := string(b)
	return strings.Contains(s, text)

}

/*RenameLocalSettings ...*/
func  (m MediaWiki) RenameLocalSettings() {
	const layout = "2006-01-02T15:04:05-0700"

	t := time.Now()
	localSettingsName := "LocalSettings-" + t.Format(layout) + ".php"

	err := os.Rename("LocalSettings.php", localSettingsName)

	if err != nil {
		log.Fatal(err)
	}
}

/*DeleteCache ...*/
func  (m MediaWiki) DeleteCache() {
	err := os.Rename("cache/.htaccess", ".htaccess.fromcache.tmp")
	if err != nil {
		log.Fatal(err)
	}

	err = os.RemoveAll("./cache/")
	if err != nil {
		log.Fatal(err)
	}

	err = os.Mkdir("cache", 0700)
	if err != nil {
		log.Fatal(err)
	}

	err = os.Rename(".htaccess.fromcache.tmp", "cache/.htaccess")
	if err != nil {
		log.Fatal(err)
	}
}

/*DeleteVendor ...*/
func  (m MediaWiki) DeleteVendor() {
	err := os.RemoveAll("./vendor")
	if err != nil {
		log.Fatal(err)
	}

	err = os.Mkdir("vendor", 0700)
	if err != nil {
		log.Fatal(err)
	}
}