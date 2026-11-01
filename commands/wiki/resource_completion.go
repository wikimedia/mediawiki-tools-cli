package wiki

import (
	"strings"

	"github.com/spf13/cobra"
)

// Resource completion is deliberately static: entity IDs, titles and opaque
// statement hashes are not guessed, and completion never opens credentials or
// performs requests. Raw API commands retain their cached schema completion.
func registerResourceCompletions(root *cobra.Command) {
	var walk func(*cobra.Command)
	walk = func(cmd *cobra.Command) {
		if cmd.RunE != nil && cmd.ValidArgsFunction == nil {
			cmd.ValidArgsFunction = cobra.NoFileCompletions
		}
		choices := map[string][]string{
			"rank":     {"normal", "preferred", "deprecated"},
			"snaktype": {"value", "somevalue", "novalue"},
			"datatype": {"string", "external-id", "url", "commonsMedia", "wikibase-item", "wikibase-property", "monolingualtext", "quantity", "time", "globe-coordinate"},
			"combine":  {"union", "intersection"},
		}
		if flag := cmd.Flags().Lookup("output"); flag != nil {
			switch flag.DefValue {
			case "json":
				choices["output"] = []string{"json", "jq"}
			case "titles":
				choices["output"] = []string{"titles", "json", "jq", "table", "auto"}
			case "text":
				choices["output"] = []string{"text", "html", "json", "jq"}
			default:
				choices["output"] = []string{"json", "jq", "table", "auto"}
			}
		}
		for name, values := range choices {
			if cmd.Flags().Lookup(name) != nil {
				candidates := values
				_ = cmd.RegisterFlagCompletionFunc(name, func(_ *cobra.Command, _ []string, prefix string) ([]string, cobra.ShellCompDirective) {
					result := []string{}
					for _, value := range candidates {
						if strings.HasPrefix(value, prefix) {
							result = append(result, value)
						}
					}
					return result, cobra.ShellCompDirectiveNoFileComp
				})
			}
		}
		for _, child := range cmd.Commands() {
			walk(child)
		}
	}
	for _, cmd := range root.Commands() {
		if cmd.Name() == "page" || cmd.Name() == "file" || cmd.Name() == "ext" || cmd.Name() == "bulk" {
			walk(cmd)
		}
	}
}
