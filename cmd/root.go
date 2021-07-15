/*Package cmd is used for command line.

Copyright © 2020 Kosta Harlan <kosta@kostaharlan.net>

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
package cmd

import (
	"fmt"
	"os"

	"gerrit.wikimedia.org/r/mediawiki/tools/cli/internal/config"
	"gerrit.wikimedia.org/r/mediawiki/tools/cli/internal/updater"
	"github.com/spf13/cobra"
)

// Verbosity indicating verbose mode.
var Verbosity int

// NonInteractive skips prompts with a yes
var NonInteractive bool

// These vars are currently used by the docker exec command

// Detach run docker command with -d
var Detach bool

// Privileged run docker command with --privileged
var Privileged bool

// User run docker command with the specified -u
var User string

// NoTTY run docker command with -T
var NoTTY bool

// Index run the docker command with the specified --index
var Index string

// Env run the docker command with the specified env vars
var Env []string

// Workdir run the docker command with this working directory
var Workdir string

// GitCommit holds short commit hash of source tree
var GitCommit string

// GitBranch holds current branch name the code is built off
var GitBranch string

// GitState shows whether there are uncommitted changes
var GitState string

// GitSummary holds output of git describe --tags --dirty --always
var GitSummary string

// BuildDate holds RFC3339 formatted UTC date (build time)
var BuildDate string

// Version holds contents of ./VERSION file, if exists, or the value passed via the -version option
var Version string

var rootCmd = &cobra.Command{
	Use:   "mw",
	Short: "Developer utilities for working with MediaWiki",
}

func wizardDevMode() {
	c := config.LoadFromDisk()
	fmt.Println("\nYou need to choose a development environment mode in order to continue:")
	fmt.Println(" - '" + config.DevModeMwdd + "' will provide advanced CLI tooling around a new mediawiki-docker-dev inspired development environment.")
	fmt.Println("\nAs the only environment available currently, it will be set as your default dev environment (alias 'dev')")

	c.DevMode = config.DevModeMwdd
	c.WriteToDisk()
}

func wizardUpdateChannel() {
	c := config.LoadFromDisk()
	fmt.Println("\nYou need to choose an update channel in order to continue:")
	fmt.Println(" - '" + config.UpdateChannelDev + "' is the only current release channel, so will be set now.")

	c.UpdateChannel = config.UpdateChannelDev
	c.WriteToDisk()
}

/*Execute the root command*/
func Execute(GitCommitIn string, GitBranchIn string, GitStateIn string, GitSummaryIn string, BuildDateIn string, VersionIn string) {
	GitCommit = GitCommitIn
	GitBranch = GitBranchIn
	GitState = GitStateIn
	GitSummary = GitSummaryIn
	BuildDate = BuildDateIn
	Version = VersionIn

	canUpdate, nextVersionString := updater.CanUpdateDaily(Version, GitSummary, false)
	if canUpdate {
		colorReset := "\033[0m"
		colorYellow := "\033[33m"
		colorWhite := "\033[37m"
		colorCyan := "\033[36m"
		fmt.Printf(
			"\n"+colorYellow+"A new update is availbile\n"+colorCyan+"%s(%s) "+colorWhite+"-> "+colorCyan+"%s"+colorReset+"\n\n",
			Version, GitSummary, nextVersionString,
		)
	}

	// Check and set needed config values
	c := config.LoadFromDisk()
	if !config.DevModeValues.Contains(c.DevMode) {
		wizardDevMode()
		c = config.LoadFromDisk()
	}
	if !config.UpdateChannelValues.Contains(c.UpdateChannel) {
		wizardUpdateChannel()
		c = config.LoadFromDisk()
	}

	// mwdd mode
	if c.DevMode == config.DevModeMwdd {
		mwddCmd.Aliases = []string{"dev"}
		mwddCmd.Short += "\t(alias: dev)"
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
