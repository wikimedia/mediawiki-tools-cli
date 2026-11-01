package wiki

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiapi"
)

func newWikiFileInfoCmd() *cobra.Command {
	var extended bool
	out := apiOutput{}
	cmd := &cobra.Command{
		Use: "info FILE", Short: "Read current file metadata without downloading an asset", Args: cobra.ExactArgs(1), ValidArgsFunction: cobra.NoFileCompletions,
		Long:    "Return the selected wiki's native imageinfo data for the current file version, including dimensions/size, MIME type, hash and URLs. Shared-repository files are supported even if the local description page is missing. Asset URLs are printed, not fetched. --extended also requests raw/common/extended metadata; fields vary by file type and site.",
		Example: "  info 'Example.jpg'\n  info 'File:Example.jpg' --extended --output jq --format '.imageinfo[0].mime'",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := out.validate(); err != nil {
				return err
			}
			if _, err := wikiapi.FileTitle(args[0]); err != nil {
				return err
			}
			client, err := wikiAPIClient(false)
			if err != nil {
				return err
			}
			info, err := client.FileInfo(cmd.Context(), args[0], extended)
			if err != nil {
				return err
			}
			return printEntityPlan(cmd, &out, info)
		},
	}
	cmd.Flags().BoolVar(&extended, "extended", false, "Include raw/common/extended metadata (may contain HTML-valued fields)")
	out.flags(cmd)
	return cmd
}

func newWikiFileUsageCmd() *cobra.Command {
	var namespace, limit int
	out := output.Output{}
	cmd := &cobra.Command{
		Use: "usage FILE", Short: "List local pages using a file", Args: cobra.ExactArgs(1), ValidArgsFunction: cobra.NoFileCompletions,
		Long:    "List direct file usage within the selected wiki, with pagination, namespace filtering and deduplication. This does not query cross-wiki/global usage. File identity may be provided by a shared repository, but all API requests stay at the selected wiki and no assets are downloaded.",
		Example: "  usage 'Example.jpg' --namespace 0 --limit 20\n  usage 'File:Example.jpg' --output titles",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateResourceOutput(&out, true); err != nil {
				return err
			}
			if limit < 0 {
				return fmt.Errorf("--limit must be nonnegative")
			}
			if _, err := wikiapi.FileTitle(args[0]); err != nil {
				return err
			}
			var selected *int
			if cmd.Flags().Changed("namespace") {
				selected = &namespace
			}
			client, err := wikiAPIClient(false)
			if err != nil {
				return err
			}
			pages, err := client.FileUsage(cmd.Context(), args[0], selected, limit)
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
	cmd.Flags().IntVar(&namespace, "namespace", 0, "Restrict usage to this namespace ID (default: all)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum distinct pages (0 means unlimited)")
	out.AddFlagsWithOpts(cmd, output.WithDefaultTTY(output.TableType), output.WithDefaultPipe(output.JSONType), output.WithAdditionalTypes(output.Type("titles")), output.WithFilterFlagDisabled(), output.WithTableBinding(&output.TableBinding{Headings: []string{"PAGEID", "NS", "TITLE"}, ProcessObjects: func(objects interface{}, table *output.Table) {
		for _, page := range objects.([]wikiapi.Page) {
			table.AddRowS(strconv.FormatInt(page.ID, 10), strconv.Itoa(page.Namespace), page.Title)
		}
	}}))
	return cmd
}
