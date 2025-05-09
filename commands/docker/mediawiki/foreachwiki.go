package mediawiki

import (
	_ "embed"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/docker"
)

//go:embed foreachwiki.example
var exampleMediawikiForeachwiki string

//go:embed foreachwiki.long
var longMediawikiForeachwiki string

func NewMediaWikiForeachwikiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "foreachwiki [flags] [script...] -- [script flags]",
		Example: cobrautil.NormalizeExample(exampleMediawikiForeachwiki),
		Short:   "Executes a MediaWiki script using run.php in the MediaWiki container for all known wikis.",
		Long:    longMediawikiForeachwiki,
		Args:    cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().EnsureReady()
			command, env := cobrautil.CommandAndEnvFromArgs(args)
			containerID, containerIDErr := mwdd.DefaultForUser().DockerCompose().ContainerID("mediawiki")
			if containerIDErr != nil {
				panic(containerIDErr)
			}
			var dbNames []string
			for _, host := range mwdd.DefaultForUser().UsedHosts() {
				if strings.Contains(host, "mediawiki.mwdd") {
					// This is what MwddSettings.php does.
					dbNames = append(dbNames, strings.Split(host, ".")[0])
				}
			}
			maxExitCode := 0
			for _, dbName := range dbNames {
				exitCode := docker.Exec(
					containerID,
					docker.ExecOptions{
						Command: append(
							[]string{
								"bash", "-c",
								// Pipe the output to sed to prepend the dbname, like scap's foreachwiki(indblist)
								fmt.Sprintf("set -o pipefail; \"$@\" | sed -u \"s/^/%s:  /\"", strings.ReplaceAll(dbName, "\"", "\\\"")),
								"--",
								"php", "maintenance/run.php", command[0], "--wiki", dbName,
							},
							command[1:]...),
						Env:        env,
						User:       User,
						WorkingDir: "/var/www/html/w",
					})
				if maxExitCode < exitCode {
					maxExitCode = exitCode
				}
			}
			if maxExitCode != 0 {
				cmd.Root().Annotations = make(map[string]string)
				cmd.Root().Annotations["exitCode"] = strconv.Itoa(maxExitCode)
			}
		},
	}
	cmd.Flags().StringVarP(&User, "user", "u", docker.CurrentUserAndGroupForDockerExecution(), "User to run as, defaults to current OS user uid:gid")
	return cmd
}
