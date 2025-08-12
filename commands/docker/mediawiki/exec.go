package mediawiki

import (
	_ "embed"
	"strconv"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/docker"
)

//go:embed exec.example
var exampleMediawikiExec string

func NewMediaWikiExecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "exec [flags] [command...]",
		Example: cobrautil.NormalizeExample(exampleMediawikiExec),
		Short:   "Executes a command in the MediaWiki container",
		Args:    cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().EnsureReady()
			command, env := cobrautil.CommandAndEnvFromArgs(args)
			containerID, containerIDErr := mwdd.DefaultForUser().DockerCompose().ContainerID("mediawiki")
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
					"/var/www/html/w",
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
