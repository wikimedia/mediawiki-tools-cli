package cmd

import (
	_ "embed"
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/Masterminds/sprig"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmd/codesearch"
	configcmd "gitlab.wikimedia.org/repos/releng/cli/internal/cmd/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmd/debug"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmd/docker"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmd/gerrit"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmd/gitlab"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmd/help"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmd/quip"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmd/toolhub"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmd/tools"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmd/update"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmd/version"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmd/wiki"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cmd/ziki"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/eventlogging"
	"gitlab.wikimedia.org/repos/releng/cli/internal/updater"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/timers"
)

//go:embed templates/usage.txt
var usageTemplate string

// Verbosity set by the user. This is a modifier that can be added to the default logrus level.
var Verbosity int

// DoTelemetry do we want to do telemetry?
var DoTelemetry bool

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
			// If PersistentPreRun is changed in any sub commands, the RootCmd.PersistentPreRun will have to be explicitly called
			// Remove the "mw" command prefix to simplify the telemetry
			if DoTelemetry {
				eventlogging.AddCommandRunEvent(cobrautil.FullCommandStringWithoutPrefix(cmd, "mw"), cli.VersionDetails.Version)
			}
		},
	}

	defaultHelpFunc := mwcliCmd.HelpFunc()
	mwcliCmd.SetHelpFunc(func(c *cobra.Command, a []string) {
		eventlogging.AddCommandRunEvent(strings.Trim(cobrautil.FullCommandStringWithoutPrefix(c, "mw")+" --help", " "), cli.VersionDetails.Version)
		defaultHelpFunc(c, a)
	})

	// We use the default logrus level of 4(info). And will add up to 2 to that for debug and trace...
	mwcliCmd.PersistentFlags().CountVarP(&Verbosity, "verbose", "v", "Increase output verbosity. Example: --verbose=2 or -vv")

	mwcliCmd.PersistentFlags().BoolVarP(&cli.Opts.NoInteraction, "no-interaction", "", false, "Do not ask any interactive questions")
	// Remove the -h help shorthand, as gitlab auth login uses it for hostname
	mwcliCmd.PersistentFlags().BoolP("help", "", false, "Help for this command")

	mwcliCmd.AddCommand([]*cobra.Command{
		codesearch.NewCodeSearchCmd(),
		configcmd.NewConfigCmd(),
		debug.NewDebugCmd(),
		toolhub.NewToolHubCmd(),
		tools.NewToolsCmd(),
		gitlab.NewGitlabCmd(),
		gerrit.NewGerritCmd(),
		docker.NewCmd(),
		update.NewUpdateCmd(),
		version.NewVersionCmd(),
		wiki.NewWikiCmd(),
		ziki.NewZikiCmd(),
		quip.NewQuipCmd(),
		help.NewOutputTopicCmd(),
	}...)

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
	lastEmittedEventTime, parseError := timers.Parse(c.TimerLastEmmitedEvent)
	if parseError != nil {
		logrus.Warn("Failed to parse last emitted event time")
	}
	if parseError == nil && timers.IsHoursAgo(lastEmittedEventTime, 1) {
		c.TimerLastEmmitedEvent = timers.String(timers.NowUTC())
		eventlogging.EmitEvents()
	}

	c.WriteToDisk()
}

/*Execute the root command.*/
func Execute(GitCommit string, GitBranch string, GitState string, GitSummary string, BuildDate string, Version string) {
	cli.VersionDetails.GitCommit = GitCommit
	cli.VersionDetails.GitBranch = GitBranch
	cli.VersionDetails.GitState = GitState
	cli.VersionDetails.GitSummary = GitSummary
	cli.VersionDetails.BuildDate = BuildDate
	cli.VersionDetails.Version = Version

	// Check and set needed config values from various wizards
	c := config.LoadFromDisk()
	if !config.DevModeValues.Contains(c.DevMode) {
		wizardDevMode(&c)
	}
	if c.Telemetry == "" {
		if cli.Opts.NoInteraction {
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
		lastUpdateCheckedTime, parseError := timers.Parse(c.TimerLastUpdateChecked)
		if parseError != nil {
			logrus.Warn("Failed to parse last update checked time")
		}
		if parseError == nil && timers.IsHoursAgo(lastUpdateCheckedTime, 3) {
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
	// Override the UsageTemplate so that:
	// - Indenting of usage examples is consistent automatically
	cobra.AddTemplateFuncs(sprig.TxtFuncMap())
	rootCmd.SetUsageTemplate(usageTemplate)

	// Execute the root command
	err := rootCmd.Execute()

	// Try and emit events after main command execution
	// TODO perhaps moved this to a POST command run thing
	if DoTelemetry {
		tryToEmitEvents()
	}

	if err != nil {
		logrus.Error(err)
		os.Exit(1)
	}
}
