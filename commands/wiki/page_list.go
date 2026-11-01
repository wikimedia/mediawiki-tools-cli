package wiki

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiapi"
)

func NewWikiPageListCmd() *cobra.Command {
	var search, category, backlinks, transclusions, titleFile, prefix, grep string
	var namespace, limit, minsize, maxsize int
	var dryRun bool
	compatibility := pywikibotSelectorCompatibility{}
	values := map[string]*string{}
	out := output.Output{}
	cmd := &cobra.Command{
		Use: "list", Short: "Select and list wiki pages",
		Long:    "List pages from one source: all pages (the default), search, category members, backlinks, transclusions, or a plain title file. Pagination and deduplication are shared across sources. Missing or invalid file titles are skipped. --namespace filters results and --limit counts distinct matching pages (0 is unlimited). The default output remains one title per line. No pages are edited.",
		Example: "  list --search 'solar eclipse' --limit 20\n  list --category Physics --namespace 0 --output json\n  list --title-file - --limit 10",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateResourceOutput(&out, true); err != nil {
				return err
			}
			options := wikiapi.PageSelection{Limit: limit, Grep: grep}
			if cmd.Flags().Changed("namespace") {
				options.Namespace = &namespace
			}
			for name, value := range map[string]string{"search": search, "category": category, "backlinks": backlinks, "transclusions": transclusions, "title-file": titleFile} {
				if cmd.Flags().Changed(name) && strings.TrimSpace(value) == "" {
					return fmt.Errorf("--%s cannot be empty", name)
				}
			}
			selectors := map[string]string{"search": search, "category": category, "backlinks": backlinks, "transclusions": transclusions, "titles": titleFile}
			for key, value := range selectors {
				if value != "" {
					if options.Source != "" {
						return fmt.Errorf("choose one page source")
					}
					options.Source, options.Value = key, value
				}
			}
			if err := compatibility.apply(cmd, &options); err != nil {
				return err
			}
			if options.Limit < 0 {
				return fmt.Errorf("--limit must be nonnegative")
			}
			if options.Source != "" {
				for _, name := range []string{"prefix", "from", "to", "filterredir", "filterlanglinks", "minsize", "maxsize", "prtype", "prlevel", "prfiltercascade", "prexpiry", "dir"} {
					if cmd.Flags().Changed(name) {
						return fmt.Errorf("--%s applies only to the default allpages source", name)
					}
				}
			}
			options.AllPages = make(map[string][]string)
			if prefix != "" {
				options.AllPages.Set("apprefix", prefix)
			}
			for name, value := range values {
				if *value != "" {
					options.AllPages.Set("ap"+name, *value)
				}
			}
			if dir := strings.ToLower(options.AllPages.Get("apdir")); dir == "asc" {
				options.AllPages.Set("apdir", "ascending")
			} else if dir == "desc" {
				options.AllPages.Set("apdir", "descending")
			}
			if minsize > 0 {
				options.AllPages.Set("apminsize", strconv.Itoa(minsize))
			}
			if maxsize > 0 {
				options.AllPages.Set("apmaxsize", strconv.Itoa(maxsize))
			}
			if dryRun {
				data, _ := json.Marshal(options)
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}
			if options.Source == "titles" {
				var input io.Reader = cmd.InOrStdin()
				if titleFile != "-" {
					file, err := os.Open(titleFile)
					if err != nil {
						return err
					}
					defer file.Close()
					input = file
				}
				titles, err := wikiapi.ReadTitles(input)
				if err != nil {
					return err
				}
				options.Titles = titles
			}
			client, err := wikiAPIClient(false)
			if err != nil {
				return err
			}
			pages, err := client.SelectPages(cmd.Context(), options)
			if err != nil {
				return err
			}
			if out.Type == "titles" {
				for _, page := range pages {
					fmt.Fprintln(cmd.OutOrStdout(), page.Title)
				}
				return nil
			}
			out.Print(cmd, pages)
			return nil
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&prefix, "prefix", "", "Prefix for the default allpages source")
	flags.IntVar(&namespace, "namespace", 0, "Namespace ID (default: main namespace for allpages, all namespaces for other sources)")
	flags.IntVar(&limit, "limit", 0, "Maximum distinct matching pages (0 means unlimited)")
	flags.StringVar(&search, "search", "", "Select pages by wiki search query")
	flags.StringVar(&category, "category", "", "Select direct category members (no recursive subcategories)")
	flags.StringVar(&backlinks, "backlinks", "", "Select pages linking to this title")
	flags.StringVar(&transclusions, "transclusions", "", "Select pages transcluding this title")
	flags.StringVar(&titleFile, "title-file", "", "Read one plain title per line from a file, or - for stdin")
	_ = cmd.MarkFlagFilename("title-file")
	compatibility.register(cmd)
	flags.StringVar(&grep, "grep", "", "Only include titles containing this literal string")
	flags.BoolVar(&dryRun, "dry-run", false, "Print the selection parameters without reading files or contacting the wiki")
	flags.IntVar(&minsize, "minsize", 0, "Minimum allpages content size in bytes")
	flags.IntVar(&maxsize, "maxsize", 0, "Maximum allpages content size in bytes")
	for _, flag := range []struct{ name, value, help string }{
		{"from", "", "First allpages title"}, {"to", "", "Last allpages title"}, {"filterredir", "all", "Allpages redirects: all, nonredirects, redirects"}, {"filterlanglinks", "all", "Allpages language links: all, withlanglinks, withoutlanglinks"}, {"prtype", "", "Allpages protection type"}, {"prlevel", "", "Allpages protection level"}, {"prfiltercascade", "all", "Allpages cascading protection filter"}, {"prexpiry", "all", "Allpages protection expiry filter"}, {"dir", "ascending", "Allpages order: ascending or descending"},
	} {
		value := new(string)
		values[flag.name] = value
		flags.StringVar(value, flag.name, flag.value, flag.help)
	}
	out.AddFlagsWithOpts(cmd, output.WithDefaultOutput(output.Type("titles")), output.WithAdditionalTypes(output.Type("titles")), output.WithFilterFlagDisabled(), output.WithTableBinding(&output.TableBinding{Headings: []string{"PAGEID", "NS", "TITLE"}, ProcessObjects: func(objects interface{}, table *output.Table) {
		for _, page := range objects.([]wikiapi.Page) {
			table.AddRowS(strconv.FormatInt(page.ID, 10), strconv.Itoa(page.Namespace), page.Title)
		}
	}}))
	return cmd
}
