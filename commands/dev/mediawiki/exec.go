package mediawiki

import (
	_ "embed"
	"strconv"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
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
		Long:    "Runs a command in the MediaWiki container with the MediaWiki source directory as its working directory. Pass the command and its arguments after `--`; use `--user` to select the container user.",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := devenv.DefaultForUser().EnsureReady(); err != nil {
				return err
			}
			command, env := cobrautil.CommandAndEnvFromArgs(args)
			containerID, containerIDErr := devenv.DefaultForUser().DockerCompose().ContainerID("mediawiki")
			if containerIDErr != nil {
				return containerIDErr
			}
			exitCode := docker.Exec(
				containerID,
				applyRelevantMediawikiWorkingDirectory(
					docker.ExecOptions{
						Command: withComposerHome(command),
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
			return nil
		},
	}
	cmd.Flags().StringVarP(&User, "user", "u", docker.CurrentUserAndGroupForDockerExecution(), "User to run as, defaults to current OS user uid:gid")
	return cmd
}
