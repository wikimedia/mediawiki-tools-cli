package mediawiki

import (
	_ "embed"
	"strconv"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
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
			mwdd.DefaultForUser().UpDetached([]string{"mediawiki-quibble"}, false)
			command, env := mwdd.CommandAndEnvFromArgs(args)
			exitCode := mwdd.DefaultForUser().DockerExec(
				applyRelevantMediawikiWorkingDirectory(
					mwdd.DockerExecCommand{
						DockerComposeService: "mediawiki-quibble",
						Command:              command,
						Env:                  env,
						User:                 User,
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
	cmd.Flags().StringVarP(&User, "user", "u", mwdd.UserAndGroupForDockerExecution(), "User to run as, defaults to current OS user uid:gid")
	return cmd
}
