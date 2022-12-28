package docker

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
)

//go:embed long/mwdd_docker-compose.md
var mwddDockerCompose string

func NewDockerComposerCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "docker-compose [flags] [docker-compose command] -- [docker-compose flags]",
		Example: "docker-compose ps\ndocker-compose -v=2 ps\ndocker-compose -v=2 ps -- --services",
		Aliases: []string{"dc"},
		Short:   "Interact directly with docker-compose",
		Long:    cli.RenderMarkdown(mwddDockerCompose),
		Run: func(cmd *cobra.Command, args []string) {
			dev := mwdd.DefaultForUser()
			dev.EnsureReady()

			// This could be simpler if the mwdd.DockerComposeCommand function just took a list of strings...
			command := ""
			if len(args) >= 1 {
				command = args[0]
			}
			commandArgs := []string{}
			if len(args) > 1 {
				commandArgs = args[1:]
			}

			mwdd.DockerComposeCommand{
				MWDD:             dev,
				Command:          command,
				CommandArguments: commandArgs,
			}.RunTTY()
		},
	}
}
