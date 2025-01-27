package mediawiki

import (
	_ "embed"
	"strconv"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/docker"
)

//go:embed mwscript.example
var exampleMediawikiMWScript string

func NewMediaWikiMWScriptCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "mwscript [flags] [script...] -- [script flags]",
		Example: exampleMediawikiMWScript,
		Short:   "Executes a MediaWiki script using run.php in the MediaWiki container",
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
				docker.ExecOptions{
					Command:    append([]string{"php", "maintenance/run.php"}, command...),
					Env:        env,
					User:       User,
					WorkingDir: "/var/www/html/w",
				},
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
