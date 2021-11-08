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

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/config"
	"gitlab.wikimedia.org/releng/cli/internal/eventlogging"
	"gitlab.wikimedia.org/releng/cli/internal/updater"
)

// These vars are currently used by the docker exec command

// Detach run docker command with -d.
var Detach bool

// Privileged run docker command with --privileged.
var Privileged bool

// User run docker command with the specified -u.
var User string

// NoTTY run docker command with -T.
var NoTTY bool

// Index run the docker command with the specified --index.
var Index string

// Env run the docker command with the specified env vars.
var Env []string

// Workdir run the docker command with this working directory.
var Workdir string

type GlobalOptions struct {
	Verbosity     int
	NoInteraction bool
}

var globalOpts GlobalOptions

type VersionAttributes struct {
	GitCommit  string // holds short commit hash of source tree.
	GitBranch  string // holds current branch name the code is built off.
	GitState   string // shows whether there are uncommitted changes.
	GitSummary string // holds output of git describe --tags --dirty --always.
	BuildDate  string // holds RFC3339 formatted UTC date (build time).
	Version    string // hold contents of ./VERSION file, if exists, or the value passed via the -version option.
}

var VersionDetails VersionAttributes

var rootCmd = &cobra.Command{
	Use:   "mw",
	Short: "Developer utilities for working with MediaWiki and Wikimedia services.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		eventlogging.RecordCommand(cmd, args)
	},
}

func init() {
	rootCmd.PersistentFlags().IntVarP(&globalOpts.Verbosity, "verbosity", "v", 1, "verbosity level (1-2)")
	rootCmd.PersistentFlags().BoolVarP(&globalOpts.NoInteraction, "no-interaction", "n", false, "Do not ask any interactive questions")
}

func wizardDevMode() {
	c := config.LoadFromDisk()
	fmt.Println("\nYou need to choose a development environment mode in order to continue:")
	fmt.Println(" - '" + config.DevModeMwdd + "' will provide advanced CLI tooling around a new mediawiki-docker-dev inspired development environment.")
	fmt.Println("\nAs the only environment available currently, it will be set as your default dev environment (alias 'dev')")

	c.DevMode = config.DevModeMwdd
	c.WriteToDisk()
}

/*Execute the root command.*/
func Execute(GitCommit string, GitBranch string, GitState string, GitSummary string, BuildDate string, Version string) {
	VersionDetails.GitCommit = GitCommit
	VersionDetails.GitBranch = GitBranch
	VersionDetails.GitState = GitState
	VersionDetails.GitSummary = GitSummary
	VersionDetails.BuildDate = BuildDate
	VersionDetails.Version = Version

	// Check and set needed config values
	c := config.LoadFromDisk()
	if !config.DevModeValues.Contains(c.DevMode) {
		wizardDevMode()
		c = config.LoadFromDisk()
	}

	// mwdd mode
	if c.DevMode == config.DevModeMwdd {
		mwddCmd.Aliases = []string{"dev"}
		mwddCmd.Short += "\t(alias: dev)"
	}

	// Check if any updates are ready for us
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

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
