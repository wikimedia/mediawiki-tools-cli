package phabricator

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/accesspolicy"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	mwconfig "gitlab.wikimedia.org/repos/releng/cli/internal/config"
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
			anonymous, err := anonymousPhabricatorRead(cmd, *site)
			if err != nil {
				return err
			}
			if anonymous {
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
	accesspolicy.Support(view)
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

func publicOnlyPhabricator(cmd *cobra.Command) (bool, error) {
	return accesspolicy.CLI(cmd, "phabricator.public_only")
}

func anonymousPhabricatorRead(cmd *cobra.Command, site string) (bool, error) {
	publicOnly, err := publicOnlyPhabricator(cmd)
	if err != nil {
		return false, err
	}
	if publicOnly {
		if cmd.Flags().Changed("site") || cmd.Flags().Changed("profile") {
			return false, fmt.Errorf("--public-only reads Wikimedia Phabricator anonymously; remove --profile or use --public-only=false")
		}
		return true, nil
	}
	return strings.TrimSpace(site) == "" && phabProfiles.Default() == "" && !mwconfig.HasCredential("phabricator.key"), nil
}

func clientFromCommand(cmd *cobra.Command, site string) (*conduitClient, *PhabConfig, error) {
	publicOnly, err := publicOnlyPhabricator(cmd)
	if err != nil {
		return nil, nil, err
	}
	if publicOnly {
		return nil, nil, fmt.Errorf("%s does not support --public-only", cmd.CommandPath())
	}
	return loadPhabricatorClient(cmd.Context(), site, true)
}

func loadPhabricatorClient(ctx context.Context, site string, useCache bool) (*conduitClient, *PhabConfig, error) {
	cfg, err := loadConfig(site)
	if err != nil {
		return nil, nil, fmt.Errorf("loading config: %w", err)
	}
	clientConfig := *cfg
	if !useCache {
		clientConfig.CachePath = ""
	}
	client := newConduitClient(&clientConfig)
	client.context = ctx
	return client, cfg, nil
}
