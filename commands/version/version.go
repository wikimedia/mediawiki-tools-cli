package version

import (
	"fmt"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

type VersionInfo struct {
	BuildDate  string
	Version    string
	Releases   string
	GitCommit  string
	GitBranch  string
	GitState   string
	GitSummary string
}

// NewVersionCmd returns the cobra command for the version command
func NewVersionCmd() *cobra.Command {
	var out = output.Output{}
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Output the version information",
		Example: `version
version --output=template --format={{.Version}}`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if output.Type(out.Type) == output.WebType {
				if cli.VersionDetails.Version == "latest" {
					fmt.Println("You are already using the latest version.")
					return nil
				}
				url := cli.VersionDetails.Version.ReleasePage()
				fmt.Println("Opening", url)
				browser.OpenURL(url)
				return nil
			}

			info := VersionInfo{
				BuildDate: cli.VersionDetails.BuildDate,
				Version:   cli.VersionDetails.Version.String(),
				Releases:  "https://gitlab.wikimedia.org/repos/releng/cli/-/releases",
			}
			if cli.Opts.Verbosity > 1 {
				info.GitCommit = cli.VersionDetails.GitCommit
				info.GitBranch = cli.VersionDetails.GitBranch
				info.GitState = cli.VersionDetails.GitState
				info.GitSummary = cli.VersionDetails.GitSummary
			}

			// Convert struct to map for output
			objects := map[interface{}]interface{}{}
			objects["BuildDate"] = info.BuildDate
			objects["Version"] = info.Version
			objects["Releases"] = info.Releases
			if cli.Opts.Verbosity > 1 {
				objects["GitCommit"] = info.GitCommit
				objects["GitBranch"] = info.GitBranch
				objects["GitState"] = info.GitState
				objects["GitSummary"] = info.GitSummary
			}

			out.Print(cmd, objects)
			return nil
		},
	}

	out.AddFlagsWithOpts(
		cmd,
		output.WithDefaultOutput(output.TableType),
		output.WithAdditionalTypes(output.WebType),
		output.WithFilterFlagDisabled(),
		output.WithTableBinding(&output.TableBinding{
			Headings: []string{"Version Information", "Value"},
			ProcessObjects: func(objects map[interface{}]interface{}, table *output.Table) {
				for key, value := range objects {
					table.AddRowS(fmt.Sprintf("%s", key), fmt.Sprintf("%s", value))
				}
			},
		}),
		output.WithAckBinding(func(objects map[interface{}]interface{}, ack *output.Ack) {
			for key, value := range objects {
				ack.AddItem("Version Information", fmt.Sprintf("%s: %s", key, value))
			}
		}),
	)

	return cmd
}
