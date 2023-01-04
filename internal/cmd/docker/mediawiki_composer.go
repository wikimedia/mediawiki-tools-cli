package docker

import (
	"strconv"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
)

func NewMediaWikiComposerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "composer",
		Short:   "Runs composer in a container in the context of MediaWiki",
		Example: "composer info\ncomposer install -- --ignore-platform-reqs",
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().EnsureReady()
			command, env := mwdd.CommandAndEnvFromArgs(args)
			exitCode := mwdd.DefaultForUser().DockerExec(
				applyRelevantMediawikiWorkingDirectory(
					mwdd.DockerExecCommand{
						DockerComposeService: "mediawiki",
						Command:              append([]string{"composer"}, command...),
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
