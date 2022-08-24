package docker

import (
	_ "embed"
	"strconv"

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
			mwdd.DefaultForUser().UpDetached([]string{"mediawiki-fresh"})
			command, env := mwdd.CommandAndEnvFromArgs(args)
			exitCode := mwdd.DefaultForUser().DockerExec(
				applyRelevantMediawikiWorkingDirectory(
					mwdd.DockerExecCommand{
						DockerComposeService: "mediawiki-fresh",
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
