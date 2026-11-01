// Limited Pywikibot page-generator syntax compatibility.
// Provenance: https://doc.wikimedia.org/pywikibot/stable/api_ref/pywikibot.pagegenerators.html
// This adapter was implemented from the documented interface; it contains no
// copied Pywikibot source and does not load or execute Python/Pywikibot scripts.
// It supports one of -search, -cat, -ref or -transcludes, one numeric -namespace,
// and -limit. -backlinks is a retained local alias for upstream's -ref spelling.
// Differences: no generator combinations, recursive categories, category start
// markers, namespace names/lists, site flags or prompts. Zero limit follows mw's
// unlimited convention. Authentication/output come from mw, not Pywikibot.
// Removal boundary: delete this file and compat_pywikibot_test.go, then remove
// the compatibility variable, apply call and register call in page_list.go.
// The canonical selectors and internal/wikiapi implementation need no changes.
package wiki

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiapi"
)

type pywikibotSelectorCompatibility struct{ args []string }

func (c *pywikibotSelectorCompatibility) register(cmd *cobra.Command) {
	cmd.Flags().StringArrayVar(&c.args, "pywikibot", nil, "Limited Pywikibot selector compatibility (repeatable; use = before -search:...)")
	cmd.Long += "\n\nPywikibot compatibility: --pywikibot accepts only -search:QUERY, -cat:TITLE, -ref:TITLE (-backlinks is a local alias), -transcludes:TITLE, -namespace:ID and -limit:N. One source and one numeric namespace only; 0 means unlimited. Recursive categories, category start markers, script execution and other Pywikibot flags are unsupported. Prefer the canonical --search/--category/--backlinks/--transclusions flags."
	cmd.Example += "\n  list --pywikibot=-ref:Earth --pywikibot=-namespace:0"
}

func (c *pywikibotSelectorCompatibility) apply(cmd *cobra.Command, options *wikiapi.PageSelection) error {
	return applyPywikibotSelectors(options, c.args, cmd.Flags().Changed("namespace"), cmd.Flags().Changed("limit"))
}

func applyPywikibotSelectors(options *wikiapi.PageSelection, args []string, explicitNamespace, explicitLimit bool) error {
	for _, arg := range args {
		name, value, ok := strings.Cut(arg, ":")
		if !ok || value == "" {
			return fmt.Errorf("unsupported Pywikibot selector %q", arg)
		}
		switch name {
		case "-namespace":
			if explicitNamespace || options.Namespace != nil {
				return fmt.Errorf("namespace specified more than once")
			}
			n, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("invalid namespace %q", value)
			}
			options.Namespace = &n
		case "-limit":
			if explicitLimit {
				return fmt.Errorf("limit specified more than once")
			}
			n, err := strconv.Atoi(value)
			if err != nil || n < 0 {
				return fmt.Errorf("invalid limit %q", value)
			}
			options.Limit = n
			explicitLimit = true
		case "-search", "-cat", "-ref", "-backlinks", "-transcludes":
			if name == "-cat" && strings.ContainsAny(value, "|#") {
				return fmt.Errorf("Pywikibot category start markers are unsupported; use --category with a plain title")
			}
			if options.Source != "" {
				return fmt.Errorf("choose one page source")
			}
			options.Source = map[string]string{"-search": "search", "-cat": "category", "-ref": "backlinks", "-backlinks": "backlinks", "-transcludes": "transclusions"}[name]
			options.Value = value
		default:
			return fmt.Errorf("unsupported Pywikibot selector %q", name)
		}
	}
	return nil
}
