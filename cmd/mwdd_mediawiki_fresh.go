package cmd

import (
	_ "embed"
	"os"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/cli"
	"gitlab.wikimedia.org/releng/cli/internal/exec"
	"gitlab.wikimedia.org/releng/cli/internal/mwdd"
)

//go:embed long/mwdd_mediawiki_fresh.md
var mwddMediawikiFreshLong string

func NewMediaWikiFreshCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fresh ...",
		Short: "Runs commands in a 'fresh' container.",
		Long:  cli.RenderMarkdown(mwddMediawikiFreshLong),
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				cmd.Help()
				os.Exit(1)
			}

			mwdd.DefaultForUser().EnsureReady()
			options := exec.HandlerOptions{
				Verbosity: globalOpts.Verbosity,
			}
			// With the current "abuse" of docker-compose to do this, we must run down before up incase the previous container failed?
			// Perhaps a better long term solution would be to NOT run up if a container exists with the correct name?
			// But then there is no way for this container to ever get updates
			// So a better solultion is to make all of this brittle
			mwdd.DefaultForUser().Rm([]string{"mediawiki-fresh"}, options)
			mwdd.DefaultForUser().UpDetached([]string{"mediawiki-fresh"}, options)
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
