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
		URL  string
	}
	out := output.Output{
		TopLevelKeys: false,
		TableBinding: &output.TableBinding{
			Headings: []string{"Host", "URL"},
			ProcessObjects: func(objects map[interface{}]interface{}, table *output.Table) {
				for _, object := range objects {
					typedObject := object.(Site)
					table.AddRowS(typedObject.Host, typedObject.URL)
				}
			},
		},
		AckBinding: func(objects map[interface{}]interface{}, ack *output.Ack) {
			for _, object := range objects {
				typedObject := object.(Site)
				ack.AddItem(typedObject.Host, fmt.Sprintf("Host: %s", typedObject.Host))
				ack.AddItem(typedObject.Host, fmt.Sprintf("URL: %s", typedObject.URL))
			}
		},
	}
	cmd := &cobra.Command{
		Use:   "sites",
		Short: "Lists sites created in your environment (since the last top level destroy command was run)",
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().EnsureReady()

			// For now just retrieve the mwdd hosts, and reverse engineer sites installed...
			objects := make(map[interface{}]interface{})
			for _, host := range mwdd.DefaultForUser().UsedHosts() {
				if strings.Contains(host, "mediawiki.mwdd") {
					objects[host] = Site{
						Host: host,
						URL:  fmt.Sprintf("http://%s:%s", host, mwdd.DefaultForUser().Env().Get("PORT")),
					}
				}
			}
			out.Print(objects)
		},
	}
	out.AddFlags(cmd, output.TableType)
	return cmd
}
