package cmd

import (
	_ "embed"
	"fmt"
	"os"
	"os/user"

	"github.com/AlecAivazis/survey/v2"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/cli"
	"gitlab.wikimedia.org/releng/cli/internal/cmd/codesearch"
	"gitlab.wikimedia.org/releng/cli/internal/cmd/debug"
	"gitlab.wikimedia.org/releng/cli/internal/cmd/gerrit"
	"gitlab.wikimedia.org/releng/cli/internal/cmd/toolhub"
	"gitlab.wikimedia.org/releng/cli/internal/cmd/wiki"
	"gitlab.wikimedia.org/releng/cli/internal/config"
	"gitlab.wikimedia.org/releng/cli/internal/eventlogging"
	"gitlab.wikimedia.org/releng/cli/internal/updater"
	cobrautil "gitlab.wikimedia.org/releng/cli/internal/util/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/util/files"
	"gitlab.wikimedia.org/releng/cli/internal/util/timers"
)

//go:embed templates/help.md
var helpTemplate string

//go:embed templates/usage.md
var usageTemplate string

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

// Verbosity set by the user. This is a modifier that can be added to the default logrus level
var Verbosity int

// DoTelemetry do we want to do telemetry?
var DoTelemetry bool

type GlobalOptions struct {
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

func NewMwCliCmd() *cobra.Command {
	mwcliCmd := &cobra.Command{
		Use:   "mw",
		Short: "Developer utilities for working with MediaWiki and Wikimedia services.",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			logrus.SetLevel(logrus.Level(int(logrus.InfoLevel) + Verbosity))
			logrus.SetFormatter(&logrus.TextFormatter{
				DisableTimestamp:       true,
				DisableLevelTruncation: true,
			})
			logrus.Trace("mwcli: PersistentPreRun")

			// All commands will call the RootCmd.PersistentPreRun, so that their commands are logged
			// If PersistentPreRun is changed in any sub commands, the RootCmd.PersistentPreRun will have to be explicity called
			// Remove the "mw" command prefix to simplify the telemetry
			if DoTelemetry {
				eventlogging.AddCommandRunEvent(cobrautil.FullCommandStringWithoutPrefix(cmd, "mw"), VersionDetails.Version)
			}
		},
	}

	// We use the default logrus level of 4(info). And will add up to 2 to that for debug and trace...
	mwcliCmd.PersistentFlags().IntVarP(&Verbosity, "verbosity", "", 0, "verbosity level (0-2)")

	mwcliCmd.PersistentFlags().BoolVarP(&globalOpts.NoInteraction, "no-interaction", "", false, "Do not ask any interactive questions")
	// Remove the -h help shorthand, as gitlab auth login uses it for hostname
	mwcliCmd.PersistentFlags().BoolP("help", "", false, "help for this command")

	// TODO down this tree we still reuse commands between instantiations of the mwcliCmd
	// Perhaps we should new everything in this call...
	cmds := []*cobra.Command{
		codesearch.NewCodeSearchCmd(),
		configAttachToCmd(),
		debug.NewDebugCmd(),
		toolhub.NewToolHubCmd(),
		gitlabAttachToCmd(),
		gerrit.NewGerritCmd(),
		mwddAttachToCmd(),
		NewUpdateCmd(),
		versionCmd,
		wiki.NewWikiCmd(),
	}
	mwcliCmd.AddCommand(cmds...)

	return mwcliCmd
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
			canUpdate, nextVersionString := updater.CanUpdate(Version, GitSummary)
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

	// mwdd mode
	if c.DevMode == config.DevModeMwdd {
		cli.MwddIsDevAlias = true
	}

	// TODO possibly move this to cli.DoTelemetry (along with verbosity?)
	DoTelemetry = c.Telemetry == "yes"

	rootCmd := NewMwCliCmd()
	rootCmd.SetUsageTemplate(usageTemplate)
	rootCmd.SetHelpTemplate(helpTemplate)

	// Execute the root command
	err := rootCmd.Execute()

	// Try and emit events after main command execution
	// TODO perhaps moved this to a POST command run thing
	if DoTelemetry {
		tryToEmitEvents()
	}

	// Perform some temporary cleanup
	{
		currentUser, _ := user.Current()
		// In 0.8.1 and before, this was the location of the last udpate time, it was since moved to the config
		files.RemoveIfExists(currentUser.HomeDir + string(os.PathSeparator) + ".mwcli/.lastUpdateCheck")
	}

	if err != nil {
		logrus.Error(err)
		os.Exit(1)
	}
}
