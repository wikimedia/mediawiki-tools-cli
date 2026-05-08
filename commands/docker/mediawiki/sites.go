package mediawiki

import (
	"fmt"
	"sort"
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
	out := output.Output{}
	cmd := &cobra.Command{
		Use:   "sites",
		Short: "Lists sites created in your environment (since the last top level destroy command was run)",
		Example: cobrautil.NormalizeExample(`
		sites
		sites --output json
		sites --output names
		sites --output template --format '{{range $k, $v := .}}{{$v.Name}}{{"\n"}}{{end}}'
	`),
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().EnsureReady()

			// For now just retrieve the mwdd hosts, and reverse engineer sites installed...
			objects := make(map[interface{}]interface{})
			for _, host := range mwdd.DefaultForUser().UsedHosts() {
				if strings.Contains(host, "mediawiki.local.wmftest.net") {
					// Make a horrible assumption that the bit before the first dot is the site name
					name := strings.Split(host, ".")[0]
					objects[host] = Site{
						Name: name,
						Host: host,
						URL:  fmt.Sprintf("http://%s:%s", host, mwdd.DefaultForUser().Env().Get("PORT")),
					}
				}
			}

			if out.Type == "names" {
				names := make([]string, 0, len(objects))
				for _, object := range objects {
					typedObject := object.(Site)
					names = append(names, typedObject.Name)
				}
				sort.Strings(names)
				for _, name := range names {
					fmt.Fprintln(cmd.OutOrStdout(), name)
				}
				return
			}

			out.Print(cmd, objects)
		},
	}
	out.AddFlagsWithOpts(
		cmd,
		output.WithDefaultTTY(output.PrettyType),
		output.WithDefaultPipe(output.JSONType),
		output.WithAdditionalTypes("names"),
		output.WithTableBinding(&output.TableBinding{
			Headings: []string{"Name", "Host", "URL"},
			RowExtractor: func(object interface{}) []string {
				typedObject, ok := object.(Site)
				if !ok {
					return nil
				}
				return []string{typedObject.Name, typedObject.Host, typedObject.URL}
			},
		}),
		output.WithPrettyBinding(func(objects interface{}, pretty *output.Pretty) {
			for _, object := range objects.(map[interface{}]interface{}) {
				typedObject := object.(Site)
				pretty.AddItem(typedObject.Host, fmt.Sprintf("Name:  %s", typedObject.Name))
				pretty.AddItem(typedObject.Host, fmt.Sprintf("URL:   %s", typedObject.URL))
			}
		}),
	)
	return cmd
}
