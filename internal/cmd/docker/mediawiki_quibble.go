package docker

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/cli"
	"gitlab.wikimedia.org/releng/cli/internal/mwdd"
)

//go:embed long/mwdd_mediawiki_quibble.md
var mwddMediawikiQuibbleLong string

//go:embed example/mediawiki_quibble.txt
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
			// With the current "abuse" of docker-compose to do this, we must run down before up incase the previous container failed?
			// Perhaps a better long term solution would be to NOT run up if a container exists with the correct name?
			// But then there is no way for this container to ever get updates
			// So a better solultion is to make all of this brittle
			mwdd.DefaultForUser().Rm([]string{"mediawiki-quibble"})
			mwdd.DefaultForUser().UpDetached([]string{"mediawiki-quibble"})
			command, env := mwdd.CommandAndEnvFromArgs(args)
			mwdd.DefaultForUser().DockerRun(applyRelevantMediawikiWorkingDirectory(mwdd.DockerExecCommand{
				DockerComposeService: "mediawiki-quibble",
				Command:              command,
				Env:                  env,
				User:                 User,
			}, "/workspace/src"))
		},
	}
	cmd.Flags().StringVarP(&User, "user", "u", mwdd.UserAndGroupForDockerExecution(), "User to run as, defaults to current OS user uid:gid")
	return cmd
}
