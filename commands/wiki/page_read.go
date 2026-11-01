package wiki

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiapi"
)

func newWikiPageInfoCmd() *cobra.Command {
	var follow bool
	out := apiOutput{}
	cmd := &cobra.Command{
		Use: "info TITLE", Short: "Read page identity, URLs and protection metadata", Args: cobra.ExactArgs(1), ValidArgsFunction: cobra.NoFileCompletions,
		Long:    "Read metadata for one page without downloading its source. Redirect pages are inspected as pages by default; --follow-redirects asks the selected wiki to resolve them. Missing or invalid titles return an error.",
		Example: "  info Earth\n  info Earth --output jq --format .fullurl",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := out.validate(); err != nil {
				return err
			}
			if strings.TrimSpace(args[0]) == "" {
				return fmt.Errorf("page title cannot be empty")
			}
			client, err := wikiAPIClient(false)
			if err != nil {
				return err
			}
			page, err := client.PageInfo(cmd.Context(), args[0], follow)
			if err != nil {
				return err
			}
			data, err := json.Marshal(page)
			if err != nil {
				return err
			}
			return out.print(cmd, data)
		},
	}
	cmd.Flags().BoolVar(&follow, "follow-redirects", false, "Resolve redirects within the selected wiki before reading metadata")
	out.flags(cmd)
	return cmd
}

func newWikiPageHistoryCmd() *cobra.Command {
	options := wikiapi.HistoryOptions{}
	out := output.Output{}
	cmd := &cobra.Command{
		Use: "history TITLE", Short: "List revision metadata for one page", Args: cobra.ExactArgs(1), ValidArgsFunction: cobra.NoFileCompletions,
		Long:    "Read paginated revision metadata, newest first by default. This does not download revision content. --limit counts distinct revisions; 0 means unlimited. Timestamps use RFC3339, with explicit timezone offsets accepted. Suppressed user/comment fields remain marked as hidden. A redirect's own history is read rather than following its target.",
		Example: "  history Earth --limit 10\n  history Earth --order newer --start 2026-01-01T00:00:00Z --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateResourceOutput(&out, false); err != nil {
				return err
			}
			if err := options.Validate(); err != nil {
				return err
			}
			if strings.TrimSpace(args[0]) == "" {
				return fmt.Errorf("page title cannot be empty")
			}
			client, err := wikiAPIClient(false)
			if err != nil {
				return err
			}
			revisions, err := client.PageHistory(cmd.Context(), args[0], options)
			if err != nil {
				return err
			}
			out.Print(cmd, revisions)
			return nil
		},
	}
	flags := cmd.Flags()
	flags.IntVar(&options.Limit, "limit", 20, "Maximum distinct revisions (0 means unlimited)")
	flags.StringVar(&options.Order, "order", "older", "Traversal order: older (newest first) or newer (oldest first)")
	flags.StringVar(&options.Start, "start", "", "Start timestamp in RFC3339 format")
	flags.StringVar(&options.End, "end", "", "End timestamp in RFC3339 format")
	flags.StringVar(&options.User, "by-user", "", "Only revisions by this user or IP address")
	flags.StringVar(&options.ExcludeUser, "exclude-user", "", "Exclude revisions by this user or IP address")
	cmd.MarkFlagsMutuallyExclusive("by-user", "exclude-user")
	_ = cmd.RegisterFlagCompletionFunc("order", func(_ *cobra.Command, _ []string, prefix string) ([]string, cobra.ShellCompDirective) {
		return matchAPIValues([]string{"older", "newer"}, prefix), cobra.ShellCompDirectiveNoFileComp
	})
	out.AddFlagsWithOpts(cmd, output.WithDefaultTTY(output.TableType), output.WithDefaultPipe(output.JSONType), output.WithFilterFlagDisabled(), output.WithTableBinding(&output.TableBinding{Headings: []string{"REVISION", "TIMESTAMP", "USER", "SIZE", "COMMENT"}, ColumnMaxWidths: []int{0, 0, 30, 0, 70}, ProcessObjects: func(objects interface{}, table *output.Table) {
		for _, revision := range objects.([]wikiapi.Revision) {
			user, comment := revision.User, revision.Comment
			if revision.UserHidden {
				user = "[hidden]"
			}
			if revision.CommentHidden {
				comment = "[hidden]"
			}
			table.AddRowS(strconv.FormatInt(revision.ID, 10), revision.Timestamp, user, strconv.FormatInt(revision.Size, 10), comment)
		}
	}}))
	return cmd
}

func newWikiPageCategoriesCmd() *cobra.Command {
	var hidden string
	var limit int
	out := output.Output{}
	cmd := &cobra.Command{
		Use: "categories TITLE", Short: "List the categories attached to one page", Args: cobra.ExactArgs(1), ValidArgsFunction: cobra.NoFileCompletions,
		Long:    "List direct category memberships of one page, including hidden categories by default. This differs from page list --category, which selects a category's members. Results are paginated and deduplicated; neither operation recurses through category trees.",
		Example: "  categories Earth --hidden exclude\n  categories Earth --output titles",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateResourceOutput(&out, true); err != nil {
				return err
			}
			if err := wikiapi.ValidateCategoryFilter(hidden, limit); err != nil {
				return err
			}
			if strings.TrimSpace(args[0]) == "" {
				return fmt.Errorf("page title cannot be empty")
			}
			client, err := wikiAPIClient(false)
			if err != nil {
				return err
			}
			categories, err := client.PageCategories(cmd.Context(), args[0], hidden, limit)
			if err != nil {
				return err
			}
			if out.Type == "titles" {
				for _, category := range categories {
					fmt.Fprintln(cmd.OutOrStdout(), category.Title)
				}
				return nil
			}
			out.Print(cmd, categories)
			return nil
		},
	}
	cmd.Flags().StringVar(&hidden, "hidden", "all", "Hidden categories: all, only or exclude")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum distinct categories (0 means unlimited)")
	_ = cmd.RegisterFlagCompletionFunc("hidden", func(_ *cobra.Command, _ []string, prefix string) ([]string, cobra.ShellCompDirective) {
		return matchAPIValues([]string{"all", "only", "exclude"}, prefix), cobra.ShellCompDirectiveNoFileComp
	})
	out.AddFlagsWithOpts(cmd, output.WithDefaultTTY(output.TableType), output.WithDefaultPipe(output.JSONType), output.WithAdditionalTypes(output.Type("titles")), output.WithFilterFlagDisabled(), output.WithTableBinding(&output.TableBinding{Headings: []string{"CATEGORY", "HIDDEN", "SORTKEY"}, ColumnMaxWidths: []int{72, 0, 30}, ProcessObjects: func(objects interface{}, table *output.Table) {
		for _, category := range objects.([]wikiapi.PageCategory) {
			table.AddRowS(category.Title, strconv.FormatBool(category.Hidden), category.SortKeyPrefix)
		}
	}}))
	return cmd
}
