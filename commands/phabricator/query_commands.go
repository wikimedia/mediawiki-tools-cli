package phabricator

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mcpserver"
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
	opts := defaultTaskListOptions()
	out := phabOutput([]string{"Task", "Title", "Status", "Priority"}, func(objects interface{}, table *output.Table) {
		for _, task := range objects.(taskPage).Data {
			table.AddRow(taskReference(task), task.Fields.Name, task.Fields.Status.Name, task.Fields.Priority.Name)
		}
	})
	cmd := &cobra.Command{
		Annotations: commandmeta.Annotations(taskListHints(false)),
		Use:         "list", Short: "Search and page through Phabricator tasks", Long: "List tasks matching project, workboard column, text, author, assignee, priority and status filters. Each request returns up to --limit tasks, ordered by update time by default; use --order newest for most recently created tasks first. Use the returned cursor with --after or --before, or --all to fetch every page. Column membership does not imply workboard card ordering. JSON output includes data and cursor fields. By default, only open tasks are included (including in-progress and stalled tasks). Use --status all to include closed tasks. --all fetches every matching page without changing the status filter.", Example: "list --project a_wikimedia_cli\nlist --project a_wikimedia_cli --column Backlog\nlist --assigned-to me --search documentation --output json\nlist --author me --order newest --status all --limit 20 --public-only\nlist --limit 50 --after CURSOR\nlist --project a_wikimedia_cli --status all --all", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			if _, err := opts.params(); err != nil {
				return err
			}
			if err := validatePhabOutput(out); err != nil {
				return err
			}
			var run taskSearch
			if useAnonymousPhabricator(cmd, *site) {
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
			out.Print(cmd, page)
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
	addPhabOutput(cmd, out)
	return cmd
}

type taskSearch = mcpserver.Action[searchOptions, taskPage]

func taskListHints(anonymous bool) commandmeta.Hints {
	if anonymous {
		return mcpserver.ReadHints(commandmeta.DataPublic)
	}
	return mcpserver.ReadHints(commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata)
}

type taskSearchInput struct {
	searchOptions
	Limit *int `json:"limit,omitempty" jsonschema:"Results per page, from 1 to 100. Defaults to 50."`
}

func taskSearchTool(anonymous bool, search taskSearch) mcpserver.Operation {
	description := "Search Phabricator tasks using the configured account. Results can include summaries of non-public tasks visible to that account."
	if anonymous {
		description = "Search public Wikimedia Phabricator tasks. Search text and filters are sent to phabricator.wikimedia.org."
	}
	description += " Returns one page. Apply at least one filter and repeat the same filters when paging."
	run := mcpserver.Action[taskSearchInput, taskPage](func(ctx context.Context, input taskSearchInput) (taskPage, error) {
		opts := input.searchOptions
		if strings.TrimSpace(opts.Search) == "" && len(opts.Projects) == 0 && len(opts.Statuses) == 0 &&
			len(opts.Priorities) == 0 && opts.Author == "" && opts.AssignedTo == "" && !opts.Unassigned && opts.Column == "" {
			return taskPage{}, fmt.Errorf("provide a task filter")
		}
		defaults := defaultTaskListOptions()
		opts.Limit = defaults.Limit
		if opts.Order == "" {
			opts.Order = defaults.Order
		}
		if opts.Statuses == nil {
			opts.Statuses = defaults.Statuses
		}
		if input.Limit != nil {
			opts.Limit = *input.Limit
		}
		return search(ctx, opts)
	})
	return run.Tool(mcp.Tool{
		Name: "phabricator_search_tasks", Description: description,
		// Task records have a reduced JSON shape when redacted.
		OutputSchema: map[string]any{
			"type": "object", "required": []string{"data", "cursor"},
			"properties": map[string]any{
				"data":   map[string]any{"type": "array", "items": map[string]string{"type": "object"}},
				"cursor": map[string]string{"type": "object"},
			},
		},
	}, taskListHints(anonymous))
}

// Preserve the MCP disclosure scope for tasks whose public visibility cannot
// be verified, even when the selected account can read additional fields.
func limitedProfileTaskSearch(client *conduitClient) taskSearch {
	public := taskAccess{publicOnly: true, endpoint: client.access.endpoint}
	return func(ctx context.Context, opts searchOptions) (taskPage, error) {
		page, err := client.searchTasks(ctx, opts)
		if err != nil {
			return page, err
		}
		for i, task := range page.Data {
			if public.permits(task) {
				continue
			}
			page.Cursor.Before = nil
			summary := phabTask{ID: task.ID, Redacted: task.Redacted}
			summary.Fields.Name = task.Fields.Name
			summary.Fields.Status.Name = task.Fields.Status.Name
			summary.Fields.Priority.Name = task.Fields.Priority.Name
			page.Data[i] = summary
		}
		return page, nil
	}
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
	cmd.Flags().IntVar(limit, "limit", defaultSearchPageLimit, "Maximum results per page (1–100)")
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
	list := &cobra.Command{
		Annotations: commandmeta.Annotations(commandmeta.Hints{
			ToolAnnotations: commandmeta.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: true},
			DataAccess:      []commandmeta.DataAccess{commandmeta.DataPrivateMetadata},
		}),
		Use: "list", Short: "Search and page through projects", Long: "Search projects by text. JSON output includes cursor fields for subsequent pages.", Example: "list --search Wikimedia\nlist --search cli --output json", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
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
			out.Print(cmd, page)
			if out.Type == "table" || out.Type == "auto" {
				printCursor(cmd, page.Cursor)
			}
			return nil
		},
	}
	list.Flags().StringVar(&query, "search", "", "Project search text")
	addPageFlags(list, &limit, &after, &before)
	addPhabOutput(list, out)
	viewOut := phabOutput([]string{"ID", "Name", "PHID"}, func(objects interface{}, table *output.Table) {
		p := objects.(*phabProject)
		table.AddRow(p.ID, p.Fields.Name, p.PHID)
	})
	view := &cobra.Command{
		Annotations: commandmeta.Annotations(commandmeta.Hints{
			ToolAnnotations: commandmeta.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: true},
			DataAccess:      []commandmeta.DataAccess{commandmeta.DataPrivateMetadata},
		}),
		Use: "view <project>", Short: "View a project by hashtag, exact name, ID or PHID", Example: "view a_wikimedia_cli", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			if err := validatePhabOutput(viewOut); err != nil {
				return err
			}
			if useAnonymousPhabricator(cmd, *site) {
				p, err := newAnonymousPhabricator().project(cmd.Context(), args[0])
				if err != nil {
					return err
				}
				viewOut.Print(cmd, p)
				return nil
			}
			client, _, err := clientFromCommand(cmd, *site)
			if err != nil {
				return err
			}
			p, err := client.exactProject(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			viewOut.Print(cmd, p)
			return nil
		},
	}
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
	cmd := &cobra.Command{
		Annotations: commandmeta.Annotations(commandmeta.Hints{
			ToolAnnotations: commandmeta.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: true},
			DataAccess:      []commandmeta.DataAccess{commandmeta.DataPrivateMetadata},
		}),
		Use: "columns <project>", Short: "List active columns with their original names and PHIDs", Long: "List every active column in a project workboard. Use a column's exact name or PHID with task list --column. Results retain API order, which may differ from the visual board order.", Example: "columns a_wikimedia_cli\ncolumns a_wikimedia_cli --output json", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
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
			out.Print(cmd, page)
			return nil
		},
	}
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
			cmd.Example = strings.ReplaceAll(cmd.Example, "mv ", "move ")
		case "setprio":
			cmd.Use = "priority <T12345> <priority>"
			cmd.Aliases = []string{"setprio", "prio"}
			cmd.Example = strings.ReplaceAll(cmd.Example, "setprio ", "priority ")
		case "setstatus":
			cmd.Use = "status <T12345> <status>"
			cmd.Aliases = []string{"setstatus"}
			cmd.Example = strings.ReplaceAll(cmd.Example, "setstatus ", "status ")
		}
	}
	return root
}

func clientFromCommand(cmd *cobra.Command, site string) (*conduitClient, *PhabConfig, error) {
	return loadCommandClient(cmd, site, true)
}

func loadCommandClient(cmd *cobra.Command, site string, useCache bool) (*conduitClient, *PhabConfig, error) {
	access, err := taskAccessFromCommand(cmd, "")
	if err != nil {
		return nil, nil, err
	}
	cfg, err := loadConfig(site)
	if err != nil {
		return nil, nil, fmt.Errorf("loading config: %w", err)
	}
	access.endpoint = strings.TrimRight(cfg.URL, "/")
	clientConfig := *cfg
	if access.publicOnly || !useCache {
		clientConfig.CachePath = ""
	}
	client := newConduitClient(&clientConfig)
	client.context = cmd.Context()
	client.access = access
	return client, cfg, nil
}
