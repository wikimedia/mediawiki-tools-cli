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
	"path/filepath"
	"strings"
)

/*NotMediaWikiDirectory error when a directory appears to not contain MediaWiki code.*/
type NotMediaWikiDirectory struct {
	directory string
}

func (e *NotMediaWikiDirectory) Error() string {
	return e.directory + " doesn't look like a MediaWiki directory"
}

/*GuessMediaWikiDirectoryBasedOnContext ...*/
func GuessMediaWikiDirectoryBasedOnContext() string {
	suggestedMwDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	for {
		// Check if the current directory looks like core
		_, checkError := ForDirectory(suggestedMwDir)
		if checkError == nil {
			// If it does look like core, break out of the loop
			break
		}
		// Otherwise, get the parent directory to try with
		suggestedMwDir = filepath.Dir(suggestedMwDir)
		if suggestedMwDir == "/" {
			// But if we reach the root level, then provide some sensible default
			suggestedMwDir = "~/git/gerrit/mediawiki/core"
			break
		}
	}

	return suggestedMwDir
}

/*ForDirectory returns a MediaWiki for the current working directory.*/
func ForDirectory(directory string) (MediaWiki, error) {
	return MediaWiki(directory), errorIfDirectoryDoesNotLookLikeCore(directory)
}

/*ForCurrentWorkingDirectory returns a MediaWiki for the current working directory.*/
func ForCurrentWorkingDirectory() (MediaWiki, error) {
	currentWorkingDirectory, _ := os.Getwd()
	return ForDirectory(currentWorkingDirectory)
}

/*CheckIfInCoreDirectory checks that the current working directory looks like a MediaWiki directory.*/
func CheckIfInCoreDirectory() {
	_, err := ForCurrentWorkingDirectory()
	if err != nil {
		log.Fatal("❌ Please run this command within the root of the MediaWiki core repository.")
	}
}

func errorIfDirectoryMissingGitReviewForProject(directory string, expectedProject string) error {
	b, err := ioutil.ReadFile(directory + string(os.PathSeparator) + ".gitreview")
	if err != nil || !strings.Contains(string(b), "project="+expectedProject) {
		return &NotMediaWikiDirectory{directory}
	}
	return nil
}

func errorIfDirectoryDoesNotLookLikeCore(directory string) error {
	return errorIfDirectoryMissingGitReviewForProject(directory, "mediawiki/core")
}
