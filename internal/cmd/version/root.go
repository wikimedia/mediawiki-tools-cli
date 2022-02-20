package version

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

func NewVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Output the version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("mw version", cli.VersionDetails.Version, "(", cli.VersionDetails.BuildDate, ")")
			fmt.Println("https://gitlab.wikimedia.org/repos/releng/cli/-/releases")

			logrus.Debugf(`GitCommit: %s
			GitBranch: %s
			GitState: %s
			GitSummary: %s
			BuildDate: %s
			Version: %s
			`,
				cli.VersionDetails.GitCommit,
				cli.VersionDetails.GitBranch,
				cli.VersionDetails.GitState,
				cli.VersionDetails.GitSummary,
				cli.VersionDetails.BuildDate,
				cli.VersionDetails.Version,
			)
		},
	}
	return cmd
}
