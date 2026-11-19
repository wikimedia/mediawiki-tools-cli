package phabricator

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mcpserver"
)

type mcpSearchInput struct {
	Query      string   `json:"query,omitempty" jsonschema:"Search text. Optional when filtering by project."`
	Projects   []string `json:"projects,omitempty" jsonschema:"Project tags, exact names, numeric IDs, or PHIDs. Tasks must match every project."`
	Priorities []int    `json:"priorities,omitempty" jsonschema:"Numeric priorities. 100 is Unbreak Now and 80 is High."`
	Unassigned bool     `json:"unassigned,omitempty" jsonschema:"Only return tasks without an assignee."`
	Limit      *int     `json:"limit,omitempty" jsonschema:"Results per page, from 1 to 100. Defaults to 20."`
	After      string   `json:"after,omitempty" jsonschema:"Cursor returned by the previous page."`
}

type mcpSearchTask struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Status   string `json:"status,omitempty"`
	Priority string `json:"priority,omitempty"`
	URI      string `json:"uri"`
	Redacted bool   `json:"redacted,omitempty"`
}

type mcpSearchResult struct {
	Tasks      []mcpSearchTask `json:"tasks"`
	NextCursor *string         `json:"nextCursor"`
}

func newPhabricatorMCPCmd(site *string) *cobra.Command {
	annotations := commandmeta.Annotations(mcpserver.ReadHints(commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata))
	annotations[cli.SkipConfigAnnotation] = "true"
	return &cobra.Command{
		Use:         "mcp",
		Short:       "Serve Phabricator task search over MCP",
		Long:        "Start a local stdio MCP server that searches open Phabricator tasks. Reads are anonymous unless you select a saved account with --profile.",
		Example:     "mcp\nmcp --profile personal",
		Annotations: annotations,
		Args:        cobra.NoArgs,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if (cmd.Flags().Changed("profile") || cmd.Flags().Changed("site")) && strings.TrimSpace(*site) == "" {
				return fmt.Errorf("provide a saved Phabricator profile name")
			}
			if commandFlagEnabled(cmd, "allow-non-public") && *site == "" {
				return fmt.Errorf("--allow-non-public requires an explicit Phabricator profile")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			server := mcpserver.New("mw-phabricator")
			tools, err := MCPTools(cmd, *site)
			if err != nil {
				return err
			}
			for _, tool := range tools {
				tool.Register(server)
			}
			return mcpserver.Serve(cmd.Context(), server)
		},
	}
}

// MCPTools binds the handlers to anonymous access or a named account at startup.
// Anonymous tools do not load the CLI configuration.
func MCPTools(cmd *cobra.Command, profile string) ([]mcpserver.Operation, error) {
	if profile == "" {
		return []mcpserver.Operation{anonymousMCPSearch(newAnonymousPhabricator())}, nil
	}
	if err := checkPhabricatorAgentPolicy(cmd); err != nil {
		return nil, err
	}

	selected, err := phabProfiles.Selected(strings.TrimSpace(profile))
	if err != nil {
		return nil, err
	}
	access, err := taskAccessFromCommand(cmd, "")
	if err != nil {
		return nil, err
	}
	cfg, err := loadConfig(selected)
	if err != nil {
		return nil, fmt.Errorf("loading Phabricator profile: %w", err)
	}
	cfg.CachePath = ""
	client := newConduitClient(cfg)
	access.endpoint = strings.TrimRight(cfg.URL, "/")
	client.access = access
	return []mcpserver.Operation{mcpSearchOperation(
		"Search open Phabricator tasks using the configured account. Results can include non-public task data visible to that account.",
		client.baseURL, client.searchTasks, commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata,
	)}, nil
}

func anonymousMCPSearch(client anonymousPhabricator) mcpserver.Operation {
	return mcpSearchOperation(
		"Search open public Wikimedia Phabricator tasks anonymously by text, projects, priorities and assignment.",
		defaultPhabricatorURL, func(ctx context.Context, opts searchOptions) (taskPage, error) {
			opts.Project = strings.Join(opts.Projects, ",")
			priorities := make([]string, len(opts.Priorities))
			for i, priority := range opts.Priorities {
				priorities[i] = strconv.Itoa(priority)
			}
			return client.searchTasks(ctx, opts, priorities, opts.Unassigned)
		}, commandmeta.DataPublic)
}

func mcpSearchOperation(description, baseURL string, search func(context.Context, searchOptions) (taskPage, error), access ...commandmeta.DataAccess) mcpserver.Operation {
	return mcpserver.NewOperation("phabricator_search_tasks", description, mcpserver.ReadHints(access...),
		func(ctx context.Context, _ *mcp.CallToolRequest, input mcpSearchInput) (*mcp.CallToolResult, mcpSearchResult, error) {
			opts, err := input.options()
			if err != nil {
				return nil, mcpSearchResult{}, err
			}
			page, err := search(ctx, opts)
			if err != nil {
				return nil, mcpSearchResult{}, err
			}
			result := mcpSearchResult{Tasks: make([]mcpSearchTask, 0, len(page.Data)), NextCursor: page.Cursor.After}
			for _, task := range page.Data {
				ref := taskReference(task)
				result.Tasks = append(result.Tasks, mcpSearchTask{
					ID: ref, Title: task.Fields.Name, Status: task.Fields.Status.Name,
					Priority: task.Fields.Priority.Name, URI: strings.TrimRight(baseURL, "/") + "/" + ref,
					Redacted: task.Redacted,
				})
			}
			return nil, result, nil
		})
}

func (input mcpSearchInput) options() (searchOptions, error) {
	query := strings.TrimSpace(input.Query)
	if query == "" && len(input.Projects) == 0 {
		return searchOptions{}, fmt.Errorf("provide search text or a project")
	}
	for _, project := range input.Projects {
		if strings.TrimSpace(project) == "" || strings.Contains(project, ",") {
			return searchOptions{}, fmt.Errorf("provide each project as a separate, nonempty array item without commas")
		}
	}
	limit := 20
	if input.Limit != nil {
		limit = *input.Limit
	}
	if _, err := queryParams(limit, input.After, ""); err != nil {
		return searchOptions{}, err
	}
	return searchOptions{
		Projects: input.Projects, Priorities: input.Priorities, Unassigned: input.Unassigned,
		Search: query, Order: "updated", Statuses: []string{"open()"}, Limit: limit, After: input.After,
	}, nil
}
