package mediawiki

import (
	_ "embed"
	"strconv"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/docker"
)

//go:embed composer.example
var composerExample string

func NewMediaWikiComposerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "composer",
		Short:   "Runs composer in a container in the context of MediaWiki",
		Example: cobrautil.NormalizeExample(composerExample),
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
						// Composer requires $HOME to be set: Default it to / if we must
						Command: composerCommand(command...),
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

// composerCommand supplies HOME for runtimes that execute an arbitrary UID
// without a passwd entry. Positional arguments preserve spaces and shell metacharacters.
func composerCommand(args ...string) []string {
	return append([]string{"sh", "-c", `HOME=${HOME:-/} "$@"`, "--", "composer"}, args...)
}

// withComposerHome supplies Composer's required HOME when a generic exec starts it.
func withComposerHome(command []string) []string {
	if len(command) > 0 && command[0] == "composer" {
		return composerCommand(command[1:]...)
	}
	return command
}
