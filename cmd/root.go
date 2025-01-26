package cmd

import (
	_ "embed"
	"fmt"
	"os"
	"strconv"
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
	stringsutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/strings"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/timers"
)

func NewMwCliCmd() *cobra.Command {
	mwcliCmd := &cobra.Command{
		Use:           "mw",
		Short:         "Developer utilities for working with MediaWiki and Wikimedia services.",
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			logrus.SetLevel(logrus.Level(int(logrus.InfoLevel) + cli.Opts.Verbosity))
			logrus.SetFormatter(&logrus.TextFormatter{
				DisableTimestamp:       true,
				DisableLevelTruncation: true,
			})
			logrus.Trace("mwcli: PersistentPreRun")

			// Force the completion command to never ask for user input
			if cmd.Name() == "__complete" {
				cli.Opts.NoInteraction = true
			}

			// Load the config early and always
			config.Load()

			// Check and set needed config values from various wizards
			// But don't ask or output to the user, or persist the config, if we are in no-interaction mode
			if !cli.Opts.NoInteraction {
				c, _ := config.Instance()
				if c.Telemetry == "" {
					t := wizardTelemetry()
					config.PutKeyValueOnDisk("telemetry", t)
					cli.Opts.Telemetry = t == "yes"
				}
			}

			// All commands will call the RootCmd.PersistentPreRun, so that their commands are logged
			// If PersistentPreRun is changed in any sub commands, the RootCmd.PersistentPreRun will have to be explicitly called
			// Remove the "mw" command prefix to simplify the telemetry
			if cli.Opts.Telemetry {
				eventlogging.AddCommandRunEvent(cobrautil.FullCommandStringWithoutPrefix(cmd, "mw"), cli.VersionDetails.Version)
			}
		},
	}
	mwcliCmd.AddGroup(&cobra.Group{
		ID:    "dev",
		Title: "Development Commands",
	})
	mwcliCmd.AddGroup(&cobra.Group{
		ID:    "service",
		Title: "Service Commands",
	})

	defaultHelpFunc := mwcliCmd.HelpFunc()
	mwcliCmd.SetHelpFunc(func(c *cobra.Command, a []string) {
		// Check that the command being run is actually a known command (or alias)
		// Otherwise we want to output an error saying "unknown command", and do a non 0 exit
		// See https://github.com/spf13/cobra/issues/706
		mwa := "mw " + strings.Join(a, " ")
		if len(a) != 0 && !strings.Contains(mwa, "--help") && !stringsutil.StringInSlice(mwa, cobrautil.AllFullCommandStringsFromParent(mwcliCmd)) {
			logrus.Errorf("unknown command: %s", strings.Join(a, " "))
			c.Root().Annotations = make(map[string]string)
			c.Root().Annotations["exitCode"] = "1"
			return
		}

		eventlogging.AddCommandRunEvent(strings.Trim(cobrautil.FullCommandStringWithoutPrefix(c, "mw")+" --help", " "), cli.VersionDetails.Version)
		defaultHelpFunc(c, a)
	})

	// We use the default logrus level of 4(info). And will add up to 2 to that for debug and trace...
	mwcliCmd.PersistentFlags().CountVarP(&cli.Opts.Verbosity, "verbose", "v", "Increase output verbosity. Example: --verbose=2 or -vv")

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

func wizardTelemetry() string {
	// Bail early in CI, and DO NOT send telemetry
	if os.Getenv("MWCLI_CONTEXT_TEST") != "" {
		return "no"
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

	if telemetryAccept {
		return "yes"
	}
	return "no"
}

func tryToEmitEvents() {
	c, _ := config.Instance()

	// Try to emit events every 1 hour
	lastEmittedEventTime, parseError := timers.Parse(c.TimerLastEmittedEvent)
	if parseError != nil {
		logrus.Warn("Failed to parse last emitted event time")
	}
	if parseError == nil && timers.IsHoursAgo(lastEmittedEventTime, 1) {
		config.PutKeyValueOnDisk("timer_last_emitted_event", c.TimerLastEmittedEvent)
		eventlogging.EmitEvents()
	}
}

/*Execute the root command.*/
func Execute(GitCommit string, GitBranch string, GitState string, GitSummary string, BuildDate string, Version cli.Version) {
	cli.VersionDetails.GitCommit = GitCommit
	cli.VersionDetails.GitBranch = GitBranch
	cli.VersionDetails.GitState = GitState
	cli.VersionDetails.GitSummary = GitSummary
	cli.VersionDetails.BuildDate = BuildDate
	cli.VersionDetails.Version = Version

	// Migration to v0.20.0+ moves the config directory from ~/.mwcli to an XDG based path
	// This is a one time migration, and can be removed in a future release
	{
		oldConfigPath := cli.LegacyUserDirectoryPath()
		newConfigPath := cli.UserDirectoryPath()
		// If the old path exists
		if _, err := os.Stat(oldConfigPath); err == nil {
			// And the new path does not exist
			if _, err := os.Stat(newConfigPath); os.IsNotExist(err) {
				// Move the old path to the new path
				logrus.Info("Migrating config directory from " + oldConfigPath + " to " + newConfigPath)
				err := os.Rename(oldConfigPath, newConfigPath)
				if err != nil {
					logrus.Warn("Failed to migrate config directory from " + oldConfigPath + " to " + newConfigPath)
				}
			}
		}
	}

	c, _ := config.Instance()

	// Check various timers and execute tasks if needed
	{
		// Check for updates every 3 hours
		lastUpdateCheckedTime, parseError := timers.Parse(c.TimerLastUpdateChecked)
		if parseError != nil {
			logrus.Warn("Failed to parse last update checked time")
		}
		if parseError == nil && timers.IsHoursAgo(lastUpdateCheckedTime, 3) {
			config.PutKeyValueOnDisk("timer_last_update_checked", c.TimerLastUpdateChecked)
			canUpdate, nextVersionString := updater.CanUpdate(Version, GitSummary)
			if canUpdate {
				colorReset := "\033[0m"
				colorYellow := "\033[33m"
				colorWhite := "\033[37m"
				colorCyan := "\033[36m"
				fmt.Printf(
					"\n"+colorYellow+"A new update is available\n"+colorCyan+"%s(%s) "+colorWhite+"-> "+colorCyan+"%s"+colorReset+"\n\n",
					Version, GitSummary, nextVersionString,
				)
			}
		}
	}

	// mwdd mode (always...)
	cli.MwddIsDevAlias = true

	rootCmd := NewMwCliCmd()
	// Override the UsageTemplate so that:
	// - Indenting of usage examples is consistent automatically
	// - Commands can be split into sections based on annotations
	cobra.AddTemplateFuncs(sprig.TxtFuncMap())
	type CommandGroup struct {
		Name     string
		Commands []*cobra.Command
	}
	cobra.AddTemplateFunc("commandGroups", func(commands []*cobra.Command) map[string]CommandGroup {
		collectCommandsInGroup := func(commands []*cobra.Command, group string) []*cobra.Command {
			collected := []*cobra.Command{}
			for _, command := range commands {
				if command.Annotations["group"] == group {
					collected = append(collected, command)
				}
			}
			return collected
		}
		groupNames := []string{}
		groups := make(map[string]CommandGroup)
		for _, command := range commands {
			groupAnnotation := command.Annotations["group"]
			if groupAnnotation != "" && !stringsutil.StringInSlice(groupAnnotation, groupNames) {
				groupNames = append(groupNames, groupAnnotation)
				groups[groupAnnotation] = CommandGroup{
					Name:     groupAnnotation,
					Commands: collectCommandsInGroup(commands, groupAnnotation),
				}
			}
		}
		return groups
	})

	// Execute the root command
	err := rootCmd.Execute()

	// Try and emit events after main command execution
	// TODO perhaps moved this to a POST command run thing
	if cli.Opts.Telemetry {
		tryToEmitEvents()
	}

	if exitCode, ok := rootCmd.Annotations["exitCode"]; ok {
		exitCodeInt, _ := strconv.Atoi(exitCode)
		os.Exit(exitCodeInt)
	}

	if err != nil {
		logrus.Errorf("%s", err)
		os.Exit(1)
	}
}
