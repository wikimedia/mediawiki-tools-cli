package gitlab

import (
	"os"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gitlab.com/gitlab-org/cli/commands"
	"gitlab.com/gitlab-org/cli/commands/cmdutils"
	"gitlab.com/gitlab-org/cli/pkg/glinstance"
	"gitlab.com/gitlab-org/cli/pkg/iostreams"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
	stringsutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/strings"
)

func Cmd() *cobra.Command {
	glinstance.OverrideDefault("gitlab.wikimedia.org")

	io := iostreams.Init()
	cmdFactory := cmdutils.NewFactory(io, false)

	// Try to keep this version in line with the addshore fork for now...
	glabCommand := commands.NewCmdRoot(cmdFactory, "mwcli "+glabVersion(), cli.VersionDetails.BuildDate)
	glabCommand.Short = "Wikimedia Gitlab instance"
	glabCommand.Use = strings.Replace(glabCommand.Use, "glab", "gitlab", 1)
	glabCommand.GroupID = "service"
	glabCommand.Aliases = []string{"glab", "gl"}

	glabCommand.Annotations["mwcli-lint-skip"] = "yarhar"
	glabCommand.Annotations["mwcli-lint-skip-children"] = "yarhar"

	defaultHelpFunc := glabCommand.HelpFunc()
	glabCommand.SetHelpFunc(func(c *cobra.Command, a []string) {
		cli.NewEvents(cli.UserDirectoryPath()+string(os.PathSeparator)+".events").AddCommandRunEvent(strings.Trim(cobrautil.FullCommandStringWithoutPrefix(c, "mw")+" --help", " "), cli.VersionDetails.Version)
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

	return glabCommand
}

func setDefaultHostname(glabCommand *cobra.Command) {
	originalPersistentPreRunE := glabCommand.PersistentPreRunE
	glabCommand.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if hostnameFlag := cmd.Flags().Lookup("hostname"); hostnameFlag != nil && !cmd.Flags().Changed("hostname") {
			_ = cmd.Flags().Set("hostname", glinstance.OverridableDefault())
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
