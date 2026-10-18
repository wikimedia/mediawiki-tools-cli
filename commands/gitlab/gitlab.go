package gitlab

import (
	"errors"
	"os"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gitlab.com/gitlab-org/cli/commands"
	"gitlab.com/gitlab-org/cli/mwcli"
	"gitlab.com/gitlab-org/cli/pkg/glinstance"
	"gitlab.com/gitlab-org/cli/pkg/iostreams"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
	stringsutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/strings"
)

func Cmd() *cobra.Command {
	glinstance.OverrideDefault("gitlab.wikimedia.org")

	io := iostreams.Init()
	cmdFactory := mwcli.NewFactory(io)

	// Try to keep this version in line with the addshore fork for now...
	glabCommand := commands.NewCmdRoot(cmdFactory, "mwcli "+glabVersion(), cli.VersionDetails.BuildDate)
	glabCommand.Short = "Work with Wikimedia GitLab"
	glabCommand.Use = strings.Replace(glabCommand.Use, "glab", "gitlab", 1)
	glabCommand.GroupID = "service"
	glabCommand.Aliases = []string{"glab", "gl"}

	glabCommand.Annotations["mwcli-lint-skip"] = "yarhar"
	glabCommand.Annotations["mwcli-lint-skip-children"] = "yarhar"

	defaultHelpFunc := glabCommand.HelpFunc()
	glabCommand.SetHelpFunc(func(c *cobra.Command, a []string) {
		cli.Opts.Telemetry = config.State().Effective.Telemetry == "yes"
		cli.NewEvents(cli.UserDirectoryPath()+string(os.PathSeparator)+".events").AddCommandHelpEvent(c, cli.VersionDetails.Version)
		defaultHelpFunc(c, a)
	})

	// Remove all "v" shorthands for command flags recursively
	cobrautil.VisitAllSubCommands(glabCommand, func(cmd *cobra.Command) {
		originalLocalFlags := cmd.LocalFlags()
		originalPersistentFlags := cmd.PersistentFlags()

		if originalLocalFlags.ShorthandLookup("v") == nil && originalPersistentFlags.ShorthandLookup("v") == nil {
			return
		}

		cmd.ResetFlags()

		originalLocalFlags.VisitAll(func(flag *pflag.Flag) {
			if flag.Shorthand == "v" {
				flag.Shorthand = ""
			}
			cmd.Flags().AddFlag(flag)
		})

		originalPersistentFlags.VisitAll(func(flag *pflag.Flag) {
			if flag.Shorthand == "v" {
				flag.Shorthand = ""
			}
			cmd.PersistentFlags().AddFlag(flag)
		})
	})

	// Hide various built in glab commands
	toHide := []string{
		// glab does not need to be updated itself, instead mwcli would need to be updated
		"check-update",
		// issues will not be used on the Wikimedia gitlab instance
		"issue",
	}

	for _, command := range glabCommand.Commands() {
		if stringsutil.StringInSlice(command.Name(), toHide) {
			glabCommand.RemoveCommand(command)
		}

		// TODO fix this one upstream
		if command.Name() == "config" {
			command.Long = strings.Replace(command.Long, "https://gitlab.com", "https://gitlab.wikimedia.org", -1)
		}
	}

	setDefaultHostname(glabCommand)
	addPushViaAPIFlag(glabCommand)

	return glabCommand
}

func addPushViaAPIFlag(glabCommand *cobra.Command) {
	createCommand, _, err := glabCommand.Find([]string{"mr", "create"})
	if err != nil || createCommand == nil || createCommand.RunE == nil {
		return
	}

	originalRunE := createCommand.RunE
	var pushViaAPI bool
	createCommand.Flags().BoolVar(&pushViaAPI, "push-via-api", false, "Push with Git over HTTPS using your mw gl token credentials.")

	createCommand.RunE = func(cmd *cobra.Command, args []string) error {
		if !pushViaAPI {
			return originalRunE(cmd, args)
		}

		if err := cmd.Flags().Set("push", "true"); err != nil {
			return err
		}

		restore, err := useGitLabTokenForHTTPS()
		if err != nil {
			return err
		}
		defer restore()

		return originalRunE(cmd, args)
	}
}

// useGitLabTokenForHTTPS temporarily makes Git rewrite Wikimedia's SSH remote
// to HTTPS and use mw gl's stored GitLab token as its credential helper. The
// Git config environment variables are inherited by the Git push subprocess,
// so this does not change the user's remotes or saved Git configuration.
func useGitLabTokenForHTTPS() (func(), error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}

	count := 0
	if value := os.Getenv("GIT_CONFIG_COUNT"); value != "" {
		count, err = strconv.Atoi(value)
		if err != nil {
			return nil, err
		}
		if count < 0 {
			return nil, errors.New("GIT_CONFIG_COUNT cannot be negative")
		}
	}

	settings := []struct{ key, value string }{
		{"url.https://gitlab.wikimedia.org/.insteadOf", "git@gitlab-ssh.wikimedia.org:"},
		{"url.https://gitlab.wikimedia.org/.insteadOf", "ssh://git@gitlab-ssh.wikimedia.org/"},
		{"url.https://gitlab.wikimedia.org/.insteadOf", "ssh://git@gitlab-ssh.wikimedia.org:22/"},
		{"credential.helper", ""},
		{"credential.https://gitlab.wikimedia.org.helper", "!" + shellQuote(executable) + " gl auth git-credential"},
	}

	keys := []string{"GIT_CONFIG_COUNT"}
	for i := range settings {
		keys = append(keys, "GIT_CONFIG_KEY_"+strconv.Itoa(count+i), "GIT_CONFIG_VALUE_"+strconv.Itoa(count+i))
	}

	previous := make(map[string]struct {
		value string
		set   bool
	}, len(keys))
	for _, key := range keys {
		value, set := os.LookupEnv(key)
		previous[key] = struct {
			value string
			set   bool
		}{value: value, set: set}
	}
	restore := func() {
		for key, old := range previous {
			if old.set {
				_ = os.Setenv(key, old.value)
			} else {
				_ = os.Unsetenv(key)
			}
		}
	}
	for i, setting := range settings {
		if err := os.Setenv("GIT_CONFIG_KEY_"+strconv.Itoa(count+i), setting.key); err != nil {
			restore()
			return nil, err
		}
		if err := os.Setenv("GIT_CONFIG_VALUE_"+strconv.Itoa(count+i), setting.value); err != nil {
			restore()
			return nil, err
		}
	}

	if err := os.Setenv("GIT_CONFIG_COUNT", strconv.Itoa(count+len(settings))); err != nil {
		restore()
		return nil, err
	}

	return restore, nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func setDefaultHostname(glabCommand *cobra.Command) {
	originalPersistentPreRunE := glabCommand.PersistentPreRunE
	glabCommand.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if hostnameFlag := cmd.Flags().Lookup("hostname"); hostnameFlag != nil {
			hostname := hostnameFlag.Value.String()
			if !cmd.Flags().Changed("hostname") {
				hostname = glinstance.OverridableDefault()
			}
			_ = cmd.Flags().Set("hostname", mwcli.Hostname(hostname))
		}

		if originalPersistentPreRunE != nil {
			return originalPersistentPreRunE(cmd, args)
		}

		return nil
	}
}

func glabVersion() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		panic("couldn't read build info")
	}

	for _, v := range bi.Deps {
		if v.Path == "gitlab.com/gitlab-org/cli" {
			return v.Version
		}
	}

	return "unknown"
}
