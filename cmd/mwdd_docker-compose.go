package cmd

import (
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/exec"
	"gitlab.wikimedia.org/releng/cli/internal/mwdd"
)

func NewDockerComposerCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "docker-compose",
		Aliases: []string{"dc"},
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().EnsureReady()

			// This could be simpiler if the mwdd.DockerComposeCommand function just took a list of strings...
			command := ""
			if len(args) >= 1 {
				command = args[0]
			}
			commandArgs := []string{}
			if len(args) > 1 {
				commandArgs = args[1:]
			}

			mwdd.DefaultForUser().DockerComposeTTY(
				mwdd.DockerComposeCommand{
					Command:          command,
					CommandArguments: commandArgs,
					HandlerOptions: exec.HandlerOptions{
						Verbosity: globalOpts.Verbosity,
					},
				},
			)
		},
	}
}
