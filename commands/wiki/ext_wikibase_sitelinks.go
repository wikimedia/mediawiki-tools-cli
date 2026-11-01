package wiki

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiapi"
)

func newEntitySitelinkCmd() *cobra.Command {
	root := &cobra.Command{Use: "sitelink", Short: "Read or edit an item's links to wiki sites"}
	for _, single := range []bool{false, true} {
		root.AddCommand(newSitelinkReadCmd(single))
	}
	for _, remove := range []bool{false, true} {
		root.AddCommand(newSitelinkEditCmd(remove))
	}
	return root
}

func newSitelinkReadCmd(single bool) *cobra.Command {
	out := apiOutput{}
	use := "list ID"
	args := cobra.ExactArgs(1)
	example := "  list Q42"
	if single {
		use = "get ID SITE"
		args = cobra.ExactArgs(2)
		example = "  get Q42 enwiki"
	}
	cmd := &cobra.Command{Use: use, Short: "Read native sitelink JSON including badges", Args: args, Example: example, RunE: func(cmd *cobra.Command, args []string) error {
		if err := out.validate(); err != nil {
			return err
		}
		client, err := wikiAPIClient(false)
		if err != nil {
			return err
		}
		entity, err := client.Entity(cmd.Context(), "item", args[0], nil, "sitelinks", false)
		if err != nil {
			return err
		}
		links, err := wikiapi.EntitySitelinks(entity)
		if err != nil {
			return err
		}
		if single {
			link, ok := links[args[1]]
			if !ok {
				return fmt.Errorf("no sitelink for %s", args[1])
			}
			return out.print(cmd, link)
		}
		return printEntityPlan(cmd, &out, links)
	}}
	out.flags(cmd)
	return cmd
}

func newSitelinkEditCmd(remove bool) *cobra.Command {
	out := apiOutput{}
	options := entityEditFlags{}
	var badges []string
	var clearBadges bool
	use := "set ID SITE TITLE"
	args := cobra.ExactArgs(3)
	example := "  set Q42 enwiki 'Douglas Adams' --dry-run"
	if remove {
		use = "remove ID SITE"
		args = cobra.ExactArgs(2)
		example = "  remove Q42 enwiki --dry-run"
	}
	cmd := &cobra.Command{
		Use: use, Short: "Apply one revision-guarded sitelink edit", Args: args, Example: example,
		Long: "Set or remove one item sitelink. Set preserves existing badges by default. Repeated --badge explicitly replaces the complete badge set; --clear-badges explicitly removes all badges. Site IDs and allowed badges are finally validated by the selected Wikibase. Every edit uses a current revision snapshot, or requires --baserevid when supplied. Dry-run reads state without fetching a CSRF token or editing.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := out.validate(); err != nil {
				return err
			}
			title := ""
			if !remove {
				title = args[2]
			}
			if cmd.Flags().Changed("badge") && len(badges) == 0 {
				return fmt.Errorf("use --clear-badges to remove all badges")
			}
			client, err := wikiAPIClient(false)
			if err != nil {
				return err
			}
			plan, err := client.PlanSitelink(cmd.Context(), args[0], args[1], title, remove, badges, clearBadges || cmd.Flags().Changed("badge"), options.BaseRevision)
			if err != nil {
				return err
			}
			return finishWikibasePlan(cmd, &out, &options, client, plan)
		},
	}
	if !remove {
		cmd.Flags().StringArrayVar(&badges, "badge", nil, "Replace all badges with these item IDs (repeatable; default: preserve)")
		cmd.Flags().BoolVar(&clearBadges, "clear-badges", false, "Explicitly replace badges with an empty set")
		cmd.MarkFlagsMutuallyExclusive("badge", "clear-badges")
	}
	options.flags(cmd)
	out.flags(cmd)
	return cmd
}
