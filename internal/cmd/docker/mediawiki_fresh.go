package docker

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
)

//go:embed long/mwdd_mediawiki_fresh.md
var mwddMediawikiFreshLong string

func NewMediaWikiFreshCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fresh [flags] [fresh-commands]... -- [fresh-args]",
		Short: "Runs commands in a 'fresh' container.",
		Args:  cobra.MinimumNArgs(1),
		Long:  cli.RenderMarkdown(mwddMediawikiFreshLong),
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().EnsureReady()
			// With the current "abuse" of docker-compose to do this, we must run down before up incase the previous container failed?
			// Perhaps a better long term solution would be to NOT run up if a container exists with the correct name?
			// But then there is no way for this container to ever get updates
			// So a better solultion is to make all of this brittle
			mwdd.DefaultForUser().Rm([]string{"mediawiki-fresh"})
			mwdd.DefaultForUser().UpDetached([]string{"mediawiki-fresh"})
			command, env := mwdd.CommandAndEnvFromArgs(args)
			mwdd.DefaultForUser().DockerRun(applyRelevantMediawikiWorkingDirectory(mwdd.DockerExecCommand{
				DockerComposeService: "mediawiki-fresh",
				Command:              command,
				Env:                  env,
				User:                 User,
			}, "/var/www/html/w"))
		},
	}
	cmd.Flags().StringVarP(&User, "user", "u", mwdd.UserAndGroupForDockerExecution(), "User to run as, defaults to current OS user uid:gid")
	return cmd
}
