package phabricator

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

func phabOutput(headings []string, rows func(interface{}, *output.Table)) *output.Output {
	return &output.Output{TableBinding: &output.TableBinding{Headings: headings, ProcessObjects: rows}}
}

func addPhabOutput(cmd *cobra.Command, out *output.Output) {
	out.AddFlagsWithOpts(cmd, output.WithDefaultTTY(output.TableType), output.WithDefaultPipe(output.JSONType), output.WithFilterFlagDisabled())
}

func validatePhabOutput(out *output.Output) error {
	switch out.Type {
	case "auto", "table", "json", "jq":
	default:
		return fmt.Errorf("unsupported output format %q", out.Type)
	}
	if out.Type == "jq" && out.Format == "" {
		return fmt.Errorf("--output jq requires --format")
	}
	return nil
}

func newTaskListCmd(site *string) *cobra.Command {
	opts := searchOptions{Limit: 50}
	out := phabOutput([]string{"Task", "Title", "Status", "Priority"}, func(objects interface{}, table *output.Table) {
		for _, task := range objects.(taskPage).Data {
			table.AddRow(fmt.Sprintf("T%d", task.ID), task.Fields.Name, task.Fields.Status.Name, task.Fields.Priority.Name)
		}
	})
	cmd := &cobra.Command{Use: "list", Short: "Search and page through Phabricator tasks", Long: "List tasks matching project, workboard column, text, assignee and status filters. Each request returns up to --limit tasks, ordered by update time. Use the returned cursor with --after or --before, or --all to fetch every page. Column membership does not imply workboard card ordering. JSON output includes data and cursor fields. By default, only open tasks are included (including in-progress and stalled tasks). Use --status all to include closed tasks. --all fetches every matching page without changing the status filter.", Example: "list --project a_wikimedia_cli\nlist --project a_wikimedia_cli --column Backlog\nlist --assigned-to me --search documentation --output json\nlist --limit 50 --after CURSOR\nlist --project a_wikimedia_cli --status all --all", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if _, err := queryParams(opts.Limit, opts.After, opts.Before); err != nil {
			return err
		}
		if err := validatePhabOutput(out); err != nil {
			return err
		}
		if opts.Column != "" && opts.Project == "" {
			return fmt.Errorf("--column requires --project")
		}
		if opts.All && (opts.After != "" || opts.Before != "") {
			return fmt.Errorf("--all cannot be combined with a cursor")
		}
		client, _, err := clientFromCommand(cmd, *site)
		if err != nil {
			return err
		}
		defer client.cache.flush()
		page, err := client.searchTasks(cmd.Context(), opts)
		if err != nil {
			return err
		}
		if err := out.Print(cmd, page); err != nil {
			return err
		}
		if out.Type == "table" || out.Type == "auto" {
			printCursor(cmd, page.Cursor)
		}
		return nil
	}}
	cmd.Flags().StringVar(&opts.Project, "project", "", "Project hashtag, exact name, ID or PHID")
	cmd.Flags().StringVar(&opts.Column, "column", "", "Exact workboard column name or PHID (requires --project)")
	cmd.Flags().StringVar(&opts.Search, "search", "", "Full-text task search")
	cmd.Flags().StringVar(&opts.AssignedTo, "assigned-to", "", "Assignee username or me")
	cmd.Flags().StringSliceVar(&opts.Statuses, "status", []string{"open()"}, "Task status (default: open tasks; use all to include closed tasks)")
	addPageFlags(cmd, &opts.Limit, &opts.After, &opts.Before)
	cmd.Flags().BoolVar(&opts.All, "all", false, "Fetch all matching pages")
	addPhabOutput(cmd, out)
	return cmd
}

func printCursor(cmd *cobra.Command, cursor pageCursor) {
	if cursor.After != nil {
		cmd.PrintErrf("Next page: --after %s\n", *cursor.After)
	}
	if cursor.Before != nil {
		cmd.PrintErrf("Previous page: --before %s\n", *cursor.Before)
	}
}

func addPageFlags(cmd *cobra.Command, limit *int, after, before *string) {
	cmd.Flags().IntVar(limit, "limit", 50, "Maximum results per page (1–100)")
	cmd.Flags().StringVar(after, "after", "", "Fetch the page after this cursor")
	cmd.Flags().StringVar(before, "before", "", "Fetch the page before this cursor")
	cmd.MarkFlagsMutuallyExclusive("after", "before")
}

func newProjectCmd(site *string) *cobra.Command {
	root := &cobra.Command{Use: "project", Short: "Find and inspect Phabricator projects"}
	var query, after, before string
	var limit int
	out := phabOutput([]string{"ID", "Name", "PHID"}, func(objects interface{}, table *output.Table) {
		for _, p := range objects.(projectPage).Data {
			table.AddRow(p.ID, p.Fields.Name, p.PHID)
		}
	})
	list := &cobra.Command{Use: "list", Short: "Search and page through projects", Long: "Search projects by text. JSON output includes cursor fields for subsequent pages.", Example: "list --search Wikimedia\nlist --search cli --output json", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if _, err := queryParams(limit, after, before); err != nil {
			return err
		}
		if err := validatePhabOutput(out); err != nil {
			return err
		}
		client, _, err := clientFromCommand(cmd, *site)
		if err != nil {
			return err
		}
		page, err := client.searchProjects(cmd.Context(), query, limit, after, before)
		if err != nil {
			return err
		}
		if err := out.Print(cmd, page); err != nil {
			return err
		}
		if out.Type == "table" || out.Type == "auto" {
			printCursor(cmd, page.Cursor)
		}
		return nil
	}}
	list.Flags().StringVar(&query, "search", "", "Project search text")
	addPageFlags(list, &limit, &after, &before)
	addPhabOutput(list, out)
	viewOut := phabOutput([]string{"ID", "Name", "PHID"}, func(objects interface{}, table *output.Table) {
		p := objects.(*phabProject)
		table.AddRow(p.ID, p.Fields.Name, p.PHID)
	})
	view := &cobra.Command{Use: "view <project>", Short: "View a project by hashtag, exact name, ID or PHID", Example: "view a_wikimedia_cli", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if err := validatePhabOutput(viewOut); err != nil {
			return err
		}
		client, _, err := clientFromCommand(cmd, *site)
		if err != nil {
			return err
		}
		p, err := client.exactProject(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		return viewOut.Print(cmd, p)
	}}
	addPhabOutput(view, viewOut)
	root.AddCommand(list, view)
	return root
}

func newBoardCmd(site *string) *cobra.Command {
	root := &cobra.Command{Use: "board", Short: "Inspect project workboards"}
	out := phabOutput([]string{"ID", "Column", "PHID"}, func(objects interface{}, table *output.Table) {
		for _, col := range objects.(columnPage).Data {
			table.AddRow(col.ID, col.Fields.Name, col.PHID)
		}
	})
	cmd := &cobra.Command{Use: "columns <project>", Short: "List active columns with their original names and PHIDs", Long: "List every active column in a project workboard. Use a column's exact name or PHID with task list --column. Results retain API order, which may differ from the visual board order.", Example: "columns a_wikimedia_cli\ncolumns a_wikimedia_cli --output json", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if err := validatePhabOutput(out); err != nil {
			return err
		}
		client, _, err := clientFromCommand(cmd, *site)
		if err != nil {
			return err
		}
		page, err := client.listColumns(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		return out.Print(cmd, page)
	}}
	addPhabOutput(cmd, out)
	root.AddCommand(cmd)
	return root
}

func newTaskCmd(site *string) *cobra.Command {
	root := &cobra.Command{Use: "task", Short: "Search, read and manage Phabricator tasks"}
	root.AddCommand(newTaskListCmd(site), newPhabricatorCreateCmd(site), newTaskEditCmd(site))
	root.AddCommand(newPhabricatorObjectViewCmd(site, "T"))
	addCommentsCmd(root, site)
	addReadCmd(root, site)
	addAssignCmd(root, site)
	addMVCmd(root, site)
	addRMTagCmd(root, site)
	addSetPrioCmd(root, site)
	addSetStatusCmd(root, site)
	for _, cmd := range root.Commands() {
		switch cmd.Name() {
		case "mv":
			cmd.Use = "move <T12345> <column>"
			cmd.Aliases = []string{"mv"}
		case "setprio":
			cmd.Use = "priority <T12345> <priority>"
			cmd.Aliases = []string{"setprio", "prio"}
		case "setstatus":
			cmd.Use = "status <T12345> <status>"
			cmd.Aliases = []string{"setstatus"}
		}
	}
	return root
}

func clientFromCommand(cmd *cobra.Command, site string) (*conduitClient, *PhabConfig, error) {
	client, cfg, err := clientFromSite(site)
	if err == nil {
		client.context = cmd.Context()
	}
	return client, cfg, err
}
