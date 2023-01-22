package dockercompose

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
)

//go:embed docker-compose.long.md
var dockerComposeLong string

//go:embed docker-compose.example
var dockerComposeExample string

func NewCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "docker-compose [flags] [docker-compose command] -- [docker-compose flags]",
		Example: dockerComposeExample,
		Aliases: []string{"dc"},
		Short:   "Interact directly with docker-compose",
		Long:    cli.RenderMarkdown(dockerComposeLong),
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