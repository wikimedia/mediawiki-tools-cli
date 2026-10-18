package mediawiki

import (
	_ "embed"
	"strconv"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/docker"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dockercompose"
)

//go:embed fresh.long.md
var devMediawikiFreshLong string

func NewMediaWikiFreshCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "fresh [flags] [fresh-commands]... -- [fresh-args]",
		Short:   "Runs commands in a 'fresh' container.",
		Args:    cobra.MinimumNArgs(1),
		Long:    cli.RenderMarkdown(devMediawikiFreshLong),
		Example: cobrautil.NormalizeExample("fresh bash\nfresh npm ci\nfresh record tests/selenium/specs/recentchanges.js"),
		Run:     runMediaWikiFresh,
	}
	cmd.PersistentFlags().StringVarP(&User, "user", "u", docker.CurrentUserAndGroupForDockerExecution(), "User to run as, defaults to current OS user uid:gid")
	cmd.AddCommand(&cobra.Command{
		Use:     "record [spec]",
		Short:   "Record a Selenium test run as video",
		Long:    "Run npm run selenium-test with video recording enabled, a non-headless browser, and one worker. Optionally select a spec; otherwise run the full suite. Run npm ci first to install dependencies. Videos normally appear in tests/selenium/log/ in the host checkout.",
		Example: cobrautil.NormalizeExample("record\nrecord tests/selenium/specs/recentchanges.js"),
		Args:    cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			command := []string{
				"npm", "run", "selenium-test", "--",
				"--recordVideo", "--useBrowserHeadless=false", "--maxInstances=1",
			}
			if len(args) == 1 {
				command = append(command, "--spec", args[0])
			}
			runMediaWikiFresh(cmd, command)
		},
	})
	return cmd
}

func runMediaWikiFresh(cmd *cobra.Command, args []string) {
	devenv.DefaultForUser().EnsureReady()
	err := devenv.DefaultForUser().DockerCompose().Up([]string{"mediawiki-fresh"}, dockercompose.UpOptions{
		Detached: true,
	})
	if err != nil {
		panic(err)
	}
	command, env := cobrautil.CommandAndEnvFromArgs(args)
	containerID, containerIDErr := devenv.DefaultForUser().DockerCompose().ContainerID("mediawiki-fresh")
	if containerIDErr != nil {
		panic(containerIDErr)
	}
	exitCode := docker.Exec(
		containerID,
		applyRelevantMediawikiWorkingDirectory(
			docker.ExecOptions{
				Command: command,
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
}
