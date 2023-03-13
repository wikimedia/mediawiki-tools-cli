package mediawiki

import (
	_ "embed"
	"strconv"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/docker"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dockercompose"
)

//go:embed quibble.long.md
var mwddMediawikiQuibbleLong string

//go:embed quibble.example
var mediawikiQuibbleExample string

func NewMediaWikiQuibbleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "quibble [flags] [quibble-commands]... -- [quibble-args]",
		Short:   "Runs commands in a 'quibble' container.",
		Args:    cobra.MinimumNArgs(1),
		Long:    cli.RenderMarkdown(mwddMediawikiQuibbleLong),
		Example: mediawikiQuibbleExample,
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().EnsureReady()
			mwdd.DefaultForUser().DockerCompose().Up([]string{"mediawiki-quibble"}, dockercompose.UpOptions{
				Detached: true,
			})
			command, env := mwdd.CommandAndEnvFromArgs(args)
			containerID, containerIDErr := mwdd.DefaultForUser().DockerCompose().ContainerID("mediawiki-quibble")
			if containerIDErr != nil {
				panic(containerIDErr)
			}
			exitCode := docker.Exec(
				containerID,
				applyRelevantMediawikiWorkingDirectory(
					docker.ExecOptions{
						Command: command,
						Env:     env,
						User:    User,
					},
					"/workspace/src",
				),
			)
			if exitCode != 0 {
				cmd.Root().Annotations = make(map[string]string)
				cmd.Root().Annotations["exitCode"] = strconv.Itoa(exitCode)
			}
		},
	}
	cmd.Flags().StringVarP(&User, "user", "u", docker.CurrentUserAndGroupForDockerExecution(), "User to run as, defaults to current OS user uid:gid")
	return cmd
}
