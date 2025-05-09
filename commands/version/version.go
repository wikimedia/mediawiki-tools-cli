package version

import (
	"fmt"
	"reflect"

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
					fmt.Println("You are using a 'latest' which indicates you built this yourself!")
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

			out.Print(cmd, info)
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
			ProcessObjects: func(object interface{}, table *output.Table) {
				info, ok := object.(VersionInfo)
				if ok {
					val := reflect.ValueOf(info)
					typ := val.Type()
					for i := 0; i < val.NumField(); i++ {
						field := typ.Field(i)
						value := val.Field(i).Interface()
						table.AddRowS(field.Name, fmt.Sprintf("%v", value))
					}
				}

			},
		}),
		output.WithAckBinding(func(object interface{}, ack *output.Ack) {
			info, ok := object.(VersionInfo)
			if ok {
				val := reflect.ValueOf(info)
				typ := val.Type()
				for i := 0; i < val.NumField(); i++ {
					field := typ.Field(i)
					value := val.Field(i).Interface()
					ack.AddItem("Version Information", fmt.Sprintf("%s: %v", field.Name, value))
				}
			}

		}),
	)

	return cmd
}
