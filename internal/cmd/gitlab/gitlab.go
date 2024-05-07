package gitlab

import (
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gitlab.com/gitlab-org/cli/commands"
	"gitlab.com/gitlab-org/cli/commands/cmdutils"
	"gitlab.com/gitlab-org/cli/pkg/glinstance"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/eventlogging"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
	stringsutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/strings"
)

func NewGitlabCmd() *cobra.Command {
	cmdFactory := cmdutils.NewFactory()

	glinstance.OverrideDefault("gitlab.wikimedia.org")

	// Try to keep this version in line with the addshore fork for now...
	glabCommand := commands.NewCmdRoot(cmdFactory, "mwcli "+glabVersion(), cli.VersionDetails.BuildDate)
	glabCommand.Short = "Interact with the Wikimedia Gitlab instance"
	glabCommand.Use = strings.Replace(glabCommand.Use, "glab", "gitlab", 1)
	glabCommand.GroupID = "service"
	glabCommand.Aliases = []string{"glab", "gl"}

	glabCommand.Annotations["mwcli-lint-skip"] = "yarhar"
	glabCommand.Annotations["mwcli-lint-skip-children"] = "yarhar"

	defaultHelpFunc := glabCommand.HelpFunc()
	glabCommand.SetHelpFunc(func(c *cobra.Command, a []string) {
		eventlogging.AddCommandRunEvent(strings.Trim(cobrautil.FullCommandStringWithoutPrefix(c, "mw")+" --help", " "), cli.VersionDetails.Version)
		defaultHelpFunc(c, a)
	})

	// Remove all "v" shothands for command flags recursively
	cobrautil.VisitAllCommands(glabCommand, func(cmd *cobra.Command) {
		originalFlags := cmd.Flags()
		if originalFlags.ShorthandLookup("v") != nil {
			cmd.ResetFlags()
			originalFlags.VisitAll(func(flag *pflag.Flag) {
				if flag.Shorthand == "v" {
					flag.Shorthand = ""
				}
				cmd.Flags().AddFlag(flag)
			})
		}
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

	return glabCommand
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
