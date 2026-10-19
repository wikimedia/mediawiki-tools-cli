package dockercompose

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
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
		Aliases: []string{"dc", "compose", "docker-compose"},
		Short:   "Interact directly with the docker compose environment",
		Long:    cli.RenderMarkdown(dockerComposeLong),
		Args:    cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dev := devenv.DefaultForUser()
			dev.EnsureReady()
			if len(args) > 0 && (args[0] == "up" || args[0] == "create" || args[0] == "run") {
				if err := dev.PrepareGateway(); err != nil {
					return err
				}
			}
			return dev.DockerCompose().Command(args).RunAttached()
		},
	}
}
