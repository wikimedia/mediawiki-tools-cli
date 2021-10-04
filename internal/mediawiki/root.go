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
	"fmt"
	"io/ioutil"
	"os"
	osexec "os/exec"
	"strings"

	"gitlab.wikimedia.org/releng/cli/internal/exec"
)

/*MediaWiki representation of a MediaWiki install directory*/
type MediaWiki string

/*Directory the directory containing MediaWiki*/
func (m MediaWiki) Directory() string {
	return string(m)
}

/*Path within the MediaWiki directory*/
func (m MediaWiki) Path(subPath string) string {
	return m.Directory() + string(os.PathSeparator) + subPath
}

/*MediaWikiIsPresent ...*/
func (m MediaWiki) MediaWikiIsPresent() bool {
	return errorIfDirectoryDoesNotLookLikeCore(m.Directory()) == nil
}

func errorIfDirectoryDoesNotLookLikeVector(directory string) error {
	return errorIfDirectoryMissingGitReviewForProject(directory, "mediawiki/skins/Vector")
}

/*VectorIsPresent ...*/
func (m MediaWiki) VectorIsPresent() bool {
	return errorIfDirectoryDoesNotLookLikeVector(m.Path("skins/Vector")) == nil
}

func exitIfNoGit() {
	_, err := osexec.LookPath("git")
	if err != nil {
		fmt.Println("You must have git installed on your system.")
		os.Exit(1)
	}
}

/*CloneSetupOpts for use with GithubCloneMediaWiki*/
type CloneSetupOpts = struct {
	GetMediaWiki          bool
	GetVector             bool
	UseGithub             bool
	UseShallow            bool
	GerritInteractionType string
	GerritUsername        string
	Options               exec.HandlerOptions
}

/*CloneSetup provides a packages initial setup method for MediaWiki and Vector with some speedy features*/
func (m MediaWiki) CloneSetup(options CloneSetupOpts) {
	exitIfNoGit()

	startRemoteCore := "https://gerrit.wikimedia.org/r/mediawiki/core"
	startRemoteVector := "https://gerrit.wikimedia.org/r/mediawiki/skins/Vector"
	if options.UseGithub {
		startRemoteCore = "https://github.com/wikimedia/mediawiki.git"
		startRemoteVector = "https://github.com/wikimedia/Vector.git"
	}

	endRemoteCore := ""
	endRemoteVector := ""
	if options.GerritInteractionType == "http" {
		endRemoteCore = "https://gerrit.wikimedia.org/r/mediawiki/core"
		endRemoteVector = "https://gerrit.wikimedia.org/r/mediawiki/skins/Vector"
	} else if options.GerritInteractionType == "ssh" {
		endRemoteCore = "ssh://" + options.GerritUsername + "@gerrit.wikimedia.org:29418/mediawiki/core"
		endRemoteVector = "ssh://" + options.GerritUsername + "@gerrit.wikimedia.org:29418/mediawiki/skins/Vector"
	} else {
		fmt.Println("Unknown GerritInteractionType")
		os.Exit(1)
	}

	shallowOptions := ""
	if options.UseShallow {
		shallowOptions = "--depth=1"
	}

	if options.GetMediaWiki {
		exec.RunTTYCommand(options.Options, exec.Command(
			"git",
			"clone",
			shallowOptions,
			startRemoteCore,
			m.Path("")))
		if startRemoteCore != endRemoteCore {
			exec.RunTTYCommand(options.Options, exec.Command(
				"git",
				"-C", m.Path(""),
				"remote",
				"set-url",
				"origin",
				endRemoteCore))
		}
	}
	if options.GetVector {
		exec.RunTTYCommand(options.Options, exec.Command(
			"git",
			"clone",
			shallowOptions,
			startRemoteVector,
			m.Path("skins/Vector")))
		if startRemoteCore != endRemoteCore {
			exec.RunTTYCommand(options.Options, exec.Command(
				"git",
				"-C", m.Path("skins/Vector"),
				"remote",
				"set-url",
				"origin",
				endRemoteVector))
		}
	}
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
func (m MediaWiki) LocalSettingsContains(text string) bool {
	b, err := ioutil.ReadFile(m.Path("LocalSettings.php"))
	if err != nil {
		panic(err)
	}
	s := string(b)
	return strings.Contains(s, text)

}
