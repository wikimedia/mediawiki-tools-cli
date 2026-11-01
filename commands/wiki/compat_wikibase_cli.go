// Limited wikibase-cli term-command syntax compatibility.
// Provenance: https://github.com/maxlath/wikibase-cli/blob/main/docs/write_operations.md
// This is an original syntax adapter, not copied wikibase-cli source or an
// implementation of its JS templates, batch runner, config, auth or executable.
// Only set-label, set-description, add-alias and remove-alias are supported,
// with entity/language/value positional arguments and Q/P type inference.
// Alias values use upstream's pipe-separated list syntax; canonical mw alias
// commands instead accept separate arguments and preserve literal pipes.
// Differences: mw profiles/output/edit flags apply, updates capture/require a
// revision, and only items/properties are accepted. Other upstream flags fail.
// Removal boundary: remove addWikibaseCLICompatibility from ext_wikibase.go and
// delete this file plus compat_wikibase_cli_test.go. Canonical resource commands,
// transport, edit validation and their tests remain independent.
package wiki

import (
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiapi"
)

func addWikibaseCLICompatibility(cmd *cobra.Command) {
	for _, alias := range []struct{ name, term, operation string }{{"set-label", "label", "set"}, {"set-description", "description", "set"}, {"add-alias", "alias", "add"}, {"remove-alias", "alias", "remove"}} {
		cmd.AddCommand(newWikibaseCLITermCompatibility(alias.name, alias.term, alias.operation))
	}
}

func newWikibaseCLITermCompatibility(name, term, operation string) *cobra.Command {
	options := entityEditFlags{}
	out := apiOutput{}
	cmd := &cobra.Command{
		Use: name + " ID LANGUAGE VALUE", Short: "Limited wikibase-cli syntax compatibility for a term edit", Hidden: true, Args: cobra.ExactArgs(3), Example: "  " + name + " Q42 en 'Example' --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := wikiapi.EntityID("entity", args[0])
			if err != nil {
				return err
			}
			kind := "item"
			if strings.HasPrefix(id, "P") {
				kind = "property"
			}
			values := args[2:]
			if term == "alias" {
				values = strings.Split(args[2], "|")
			}
			return runEntityTermChange(cmd, &out, &options, kind, id, term, operation, args[1], values)
		},
	}
	cmd.Long = "Limited wikibase-cli syntax compatibility. Prefer item/property " + term + " " + operation + ". Only Q/P IDs and a single edit are supported; mw profiles and revision protection apply. Alias values are pipe-separated. JS templates, batches and upstream configuration flags are unsupported."
	options.flags(cmd)
	out.flags(cmd)
	return cmd
}
