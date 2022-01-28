package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Output the version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("mw version", VersionDetails.Version, "(", VersionDetails.BuildDate, ")")
		fmt.Println("https://gitlab.wikimedia.org/releng/cli/-/releases")

		if globalOpts.Verbosity > 1 {
			fmt.Printf(`GitCommit: %s
GitBranch: %s
GitState: %s
GitSummary: %s
BuildDate: %s
Version: %s
`,
				VersionDetails.GitCommit,
				VersionDetails.GitBranch,
				VersionDetails.GitState,
				VersionDetails.GitSummary,
				VersionDetails.BuildDate,
				VersionDetails.Version,
			)
		}
	},
}

func versionAttachToCmd(rootCmd *cobra.Command) {
	rootCmd.AddCommand(versionCmd)
}
