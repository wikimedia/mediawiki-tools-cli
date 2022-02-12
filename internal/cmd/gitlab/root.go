package gitlab

import (
	"runtime/debug"
	"strings"

	"github.com/profclems/glab/commands"
	"github.com/profclems/glab/commands/cmdutils"
	"github.com/profclems/glab/pkg/glinstance"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/cli"
)

func NewGitlabCmd() *cobra.Command {
	cmdFactory := cmdutils.NewFactory()

	glinstance.OverrideDefault("gitlab.wikimedia.org")

	// Try to keep this version in line with the addshore fork for now...
	glabCommand := commands.NewCmdRoot(cmdFactory, "mwcli "+glabVersion(), cli.VersionDetails.BuildDate)
	glabCommand.Short = "Wikimedia Gitlab instance"
	glabCommand.Use = strings.Replace(glabCommand.Use, "glab", "gitlab", 1)
	glabCommand.Aliases = []string{"glab"}
	glabCommand.ResetFlags()

	// Hide various built in glab commands
	toHide := []string{
		// glab does not need to be updated itself, instead mwcli would need to be updated
		"check-update",
		// issues will not be used on the Wikimedia gitlab instance
		"issue",
	}

	for _, command := range glabCommand.Commands() {
		_, shouldHide := findInSlice(toHide, command.Name())
		if shouldHide {
			glabCommand.RemoveCommand(command)
		}

		// TODO fix this one upstream
		if command.Name() == "config" {
			command.Long = strings.Replace(command.Long, "https://gitlab.com", "https://gitlab.wikimedia.org", -1)
		}
	}

	return glabCommand
}

func findInSlice(slice []string, val string) (int, bool) {
	for i, item := range slice {
		if item == val {
			return i, true
		}
	}
	return -1, false
}

func glabVersion() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		panic("couldn't read build info")
	}

	for _, v := range bi.Deps {
		if v.Path == "github.com/profclems/glab" {
			return v.Version
		}
	}

	return "unknown"
}
