package version

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

func NewVersionCmd() *cobra.Command {
	out := versionOutput()
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Output the version information",
		Run: func(cmd *cobra.Command, args []string) {
			objects := make(map[interface{}]interface{}, 7)

			if cli.Opts.Verbosity > 1 {
				objects["GitCommit"] = cli.VersionDetails.GitCommit
				objects["GitBranch"] = cli.VersionDetails.GitBranch
				objects["GitState"] = cli.VersionDetails.GitState
				objects["GitSummary"] = cli.VersionDetails.GitSummary
			}

			objects["BuildDate"] = cli.VersionDetails.BuildDate
			objects["Version"] = cli.VersionDetails.Version
			objects["Releases"] = "https://gitlab.wikimedia.org/repos/releng/cli/-/releases"

			out.Print(objects)
		},
	}

	out.AddFlags(cmd, string(output.TableType))
	return cmd
}

func versionOutput() output.Output {
	return output.Output{
		TopLevelKeys: true,
		TableBinding: &output.TableBinding{
			Headings: []string{"Version Information", "Value"},
			ProcessObjects: func(objects map[interface{}]interface{}, table *output.Table) {
				for key, value := range objects {
					table.AddRowS(fmt.Sprintf("%s", key), fmt.Sprintf("%s", value))
				}
			},
		},
		AckBinding: func(objects map[interface{}]interface{}, ack *output.Ack) {
			for key, value := range objects {
				ack.AddItem("Version Information", fmt.Sprintf("%s: %s", key, value))
			}
		},
	}
}
