package phabricator

import (
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/accesspolicy"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mcpserver"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

func newTaskListCmd(site *string) *cobra.Command {
	opts := defaultTaskListOptions()
	out := phabOutput([]string{"Task", "Title", "Status", "Priority"}, func(objects interface{}, table *output.Table) {
		for _, task := range objects.(taskPage).Data {
			table.AddRow(taskReference(task), task.Fields.Name, task.Fields.Status.Name, task.Fields.Priority.Name)
		}
	})
	cmd := &cobra.Command{
		Annotations: commandmeta.Annotations(mcpserver.ReadHints(commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata)),
		Use:         "list", Short: "Search and page through Phabricator tasks", Long: "List tasks matching project, workboard column, text, author, assignee, priority and status filters. Each request returns up to --limit tasks, ordered by update time by default; use --order newest for most recently created tasks first. Use the returned cursor with --after or --before, or --all to fetch every page. Column membership does not imply workboard card ordering. JSON output includes data and cursor fields. By default, only open tasks are included (including in-progress and stalled tasks). Use --status all to include closed tasks. --all fetches every matching page without changing the status filter.", Example: "list --project a_wikimedia_cli\nlist --project a_wikimedia_cli --column Backlog\nlist --assigned-to me --search documentation --output json\nlist --author me --order newest --status all --limit 20 --public-only\nlist --limit 50 --after CURSOR\nlist --project a_wikimedia_cli --status all --all", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			if _, err := opts.params(); err != nil {
				return err
			}
			if err := validatePhabOutput(out); err != nil {
				return err
			}
			var run taskSearch
			anonymous, err := anonymousPhabricatorRead(cmd, *site)
			if err != nil {
				return err
			}
			if anonymous {
				run = newAnonymousPhabricator().searchTasks
			} else {
				client, _, loadErr := clientFromCommand(cmd, *site)
				if loadErr != nil {
					return loadErr
				}
				defer client.cache.flush()
				run = client.searchTasks
			}
			page, err := run(cmd.Context(), opts)
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
		},
	}
	cmd.Flags().StringSliceVar(&opts.Projects, "project", nil, "Project hashtag, exact name, ID or PHID (repeatable or comma-separated)")
	cmd.Flags().StringVar(&opts.Column, "column", "", "Exact workboard column name or PHID (requires one --project and a visible workboard)")
	cmd.Flags().StringVar(&opts.Search, "search", "", "Full-text task search")
	cmd.Flags().StringVar(&opts.AssignedTo, "assigned-to", "", "Assignee username or me")
	cmd.Flags().StringVar(&opts.Author, "author", "", "Author username or me")
	cmd.Flags().StringVar(&opts.Order, "order", opts.Order, "Task ordering: updated or newest (most recently created first)")
	cmd.Flags().StringSliceVar(&opts.Statuses, "status", opts.Statuses, "Task status (default: open tasks; use all to include closed tasks)")
	cmd.Flags().IntSliceVar(&opts.Priorities, "priority", nil, "Numeric priorities (100=Unbreak Now, 80=High)")
	cmd.Flags().BoolVar(&opts.Unassigned, "unassigned", false, "Only show unassigned tasks")
	addPageFlags(cmd, &opts.Limit, &opts.After, &opts.Before)
	cmd.Flags().BoolVar(&opts.All, "all", false, "Fetch all matching pages")
	accesspolicy.Support(cmd)
	addPhabOutput(cmd, out)
	return cmd
}

type taskSearch = mcpserver.Action[searchOptions, taskPage]
