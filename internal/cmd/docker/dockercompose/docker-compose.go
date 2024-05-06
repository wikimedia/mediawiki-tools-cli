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
		Use:     "compose [flags] [compose command] -- [compose flags]",
		GroupID: "core",
		Example: dockerComposeExample,
		Aliases: []string{"dc", "compose"},
		Short:   "Interact directly with the docker compose environment",
		Long:    cli.RenderMarkdown(dockerComposeLong),
		Run: func(cmd *cobra.Command, args []string) {
			dev := mwdd.DefaultForUser()
			dev.EnsureReady()
			err := dev.DockerCompose().Command(args).RunAttached()
			if err != nil {
				panic(err)
			}
		},
	}
}
