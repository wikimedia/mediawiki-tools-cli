package mediawiki

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

func NewMediaWikiSitesCmd() *cobra.Command {
	type Site struct {
		Host string
	}
	out := output.Output{
		TopLevelKeys: false,
		TableBinding: &output.TableBinding{
			Headings: []string{"Host"},
			ProcessObjects: func(objects map[interface{}]interface{}, table *output.Table) {
				for key := range objects {
					table.AddRowS(fmt.Sprintf("%s", key))
				}
			},
		},
		AckBinding: func(objects map[interface{}]interface{}, ack *output.Ack) {
			for key := range objects {
				ack.AddItem("Host", fmt.Sprintf("%s", key))
			}
		},
	}
	cmd := &cobra.Command{
		Use:   "sites",
		Short: "Lists sites created since the last top level destroy command was run",
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().EnsureReady()

			// For now just retrieve the mwdd hosts, and reverse engineer sites installed...
			objects := make(map[interface{}]interface{})
			for _, host := range mwdd.DefaultForUser().UsedHosts() {
				if strings.Contains(host, "mediawiki.mwdd") {
					objects[host] = Site{
						Host: host,
					}
				}
			}
			out.Print(objects)
		},
	}
	out.AddFlags(cmd, "table")
	return cmd
}
