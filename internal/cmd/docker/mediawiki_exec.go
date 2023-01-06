package docker

import (
	_ "embed"
	"strconv"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
)

//go:embed example/mediawiki_exec.txt
var exampleMediawikiExec string

func NewMediaWikiExecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "exec [flags] [command...]",
		Example: exampleMediawikiExec,
		Short:   "Executes a command in the MediaWiki container",
		Args:    cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().EnsureReady()
			command, env := mwdd.CommandAndEnvFromArgs(args)
			exitCode := mwdd.DefaultForUser().DockerExec(
				applyRelevantMediawikiWorkingDirectory(
					mwdd.DockerExecCommand{
						DockerComposeService: "mediawiki",
						Command:              command,
						Env:                  env,
						User:                 User,
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
	cmd.Flags().StringVarP(&User, "user", "u", mwdd.UserAndGroupForDockerExecution(), "User to run as, defaults to current OS user uid:gid")
	return cmd
}
