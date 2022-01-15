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
	"os/user"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/config"
	"gitlab.wikimedia.org/releng/cli/internal/eventlogging"
	"gitlab.wikimedia.org/releng/cli/internal/updater"
	cobrautil "gitlab.wikimedia.org/releng/cli/internal/util/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/util/files"
	"gitlab.wikimedia.org/releng/cli/internal/util/timers"
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
}

func init() {
	rootCmd.PersistentFlags().IntVarP(&globalOpts.Verbosity, "verbosity", "v", 1, "verbosity level (1-2)")
	rootCmd.PersistentFlags().BoolVarP(&globalOpts.NoInteraction, "no-interaction", "n", false, "Do not ask any interactive questions")
}

func wizardDevMode(c *config.Config) {
	fmt.Println("\nYou need to choose a development environment mode in order to continue:")
	fmt.Println(" - '" + config.DevModeMwdd + "' will provide advanced CLI tooling around a new mediawiki-docker-dev inspired development environment.")
	fmt.Println("\nAs the only environment available currently, it will be set as your default dev environment (alias 'dev')")

	c.DevMode = config.DevModeMwdd
}

func wizardTelemetry(c *config.Config) {
	// Bail early in CI, and DO NOT send telemetry
	if os.Getenv("MWCLI_CONTEXT_TEST") != "" {
		c.Telemetry = "no"
		return
	}

	fmt.Println("\nWe would like to collect anonymous usage statistics to help improve this CLI tool.")
	fmt.Println("If you accept, these statistics will periodically be submitted to the Wikimedia event intake.")

	telemetryAccept := false
	telemetryPrompt := &survey.Confirm{
		Message: "Do you accept?",
	}
	err := survey.AskOne(telemetryPrompt, &telemetryAccept)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Record string instead of boolean, so that in the future we can re ask this question
	if telemetryAccept {
		c.Telemetry = "yes"
	} else {
		c.Telemetry = "no"
	}
}

func tryToEmitEvents() {
	c := config.LoadFromDisk()
	if c.TimerLastEmmitedEvent == "" {
		c.TimerLastEmmitedEvent = timers.String(timers.NowUTC())
	}

	// Try to emit events every 1 hour
	if timers.IsHoursAgo(timers.Parse(c.TimerLastEmmitedEvent), 1) {
		c.TimerLastEmmitedEvent = timers.String(timers.NowUTC())
		eventlogging.EmitEvents()
	}

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

	// Check and set needed config values from various wizards
	c := config.LoadFromDisk()
	if !config.DevModeValues.Contains(c.DevMode) {
		wizardDevMode(&c)
	}
	if c.Telemetry == "" {
		if globalOpts.NoInteraction {
			c.Telemetry = "no"
		} else {
			wizardTelemetry(&c)
		}
	}
	c.WriteToDisk()

	// mwdd mode
	if c.DevMode == config.DevModeMwdd {
		mwddCmd.Aliases = []string{"dev"}
		mwddCmd.Short += "\t(alias: dev)"
	}

	// Check various timers and execute tasks if needed
	{
		// Setup timers if they are not set
		if c.TimerLastUpdateChecked == "" {
			c.TimerLastUpdateChecked = timers.String(timers.NowUTC())
		}

		// Check if timers trigger things
		// Check for updates every 3 hours
		if timers.IsHoursAgo(timers.Parse(c.TimerLastUpdateChecked), 3) {
			c.TimerLastUpdateChecked = timers.String(timers.NowUTC())
			canUpdate, nextVersionString := updater.CanUpdate(Version, GitSummary, false)
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
		}

		// Write config back to disk once timers are updated
		c.WriteToDisk()
	}

	if c.Telemetry == "yes" {
		rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
			// All commands will call the RootCmd.PersistentPreRun, so that their commands are logged
			// If PersistentPreRun is changed in any sub commands, the RootCmd.PersistentPreRun will have to be explicity called
			eventlogging.AddCommandRunEvent(cobrautil.FullCommandString(cmd), VersionDetails.Version)
		}
	} else {
		// We must set a pre run comand, that does nothing, to avoidnil pointers for out sub commands that call this pre run
		rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {}
	}

	// Execute the root command
	err := rootCmd.Execute()

	// Try and emit events after main command execution
	if c.Telemetry == "yes" {
		tryToEmitEvents()
	}

	// Perform some temporary cleanup
	{
		currentUser, _ := user.Current()
		// In 0.8.1 and before, this was the location of the last udpate time, it was since moved to the config
		files.RemoveIfExists(currentUser.HomeDir + string(os.PathSeparator) + ".mwcli/.lastUpdateCheck")
	}

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
