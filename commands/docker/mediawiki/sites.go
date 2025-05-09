package mediawiki

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

func NewMediaWikiSitesCmd() *cobra.Command {
	type Site struct {
		Name string
		Host string
		URL  string
	}
	out := output.Output{
		TableBinding: &output.TableBinding{
			Headings: []string{"Name", "Host", "URL"},
			ProcessObjects: func(objects interface{}, table *output.Table) {
				for _, object := range objects.(map[interface{}]interface{}) {
					typedObject := object.(Site)
					table.AddRowS(typedObject.Name, typedObject.Host, typedObject.URL)
				}
			},
		},
		AckBinding: func(objects interface{}, ack *output.Ack) {
			for _, object := range objects.(map[interface{}]interface{}) {
				typedObject := object.(Site)
				ack.AddItem(typedObject.Host, fmt.Sprintf("Name: %s", typedObject.Name))
				ack.AddItem(typedObject.Host, fmt.Sprintf("Host: %s", typedObject.Host))
				ack.AddItem(typedObject.Host, fmt.Sprintf("URL: %s", typedObject.URL))
			}
		},
	}
	cmd := &cobra.Command{
		Use:   "sites",
		Short: "Lists sites created in your environment (since the last top level destroy command was run)",
		Example: cobrautil.NormalizeExample(`
		sites
		sites --output json --format .Name | jq -r
	`),
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().EnsureReady()

			// For now just retrieve the mwdd hosts, and reverse engineer sites installed...
			objects := make(map[interface{}]interface{})
			for _, host := range mwdd.DefaultForUser().UsedHosts() {
				if strings.Contains(host, "mediawiki.mwdd") {
					// Make a horrible assumption that the bit before the first dot is the site name
					name := strings.Split(host, ".")[0]
					objects[host] = Site{
						Name: name,
						Host: host,
						URL:  fmt.Sprintf("http://%s:%s", host, mwdd.DefaultForUser().Env().Get("PORT")),
					}
				}
			}
			out.Print(cmd, objects)
		},
	}
	out.AddFlags(cmd, output.TableType)
	return cmd
}
