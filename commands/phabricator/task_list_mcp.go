package phabricator

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mcpserver"
)

type taskSearchInput struct {
	searchOptions
	Limit *int `json:"limit,omitempty" jsonschema:"Results per page, from 1 to 100. Defaults to 50."`
}

func taskSearchTool(anonymous bool, search taskSearch) mcpserver.Operation {
	hints := mcpserver.ReadHints(commandmeta.DataPublic)
	destination := "phabricator.wikimedia.org"
	fields := []string{"search text", "filters", "sort order", "page limit", "cursor"}
	description := "Search Phabricator tasks using the configured account. Results can include summaries of non-public tasks visible to that account."
	if anonymous {
		description = "Search public Wikimedia Phabricator tasks. Search text and filters are sent to phabricator.wikimedia.org."
	} else {
		destination = "configured Phabricator site"
		hints = mcpserver.ReadHints(commandmeta.DataPublic, commandmeta.DataPrivateMetadata)
		fields = append(fields, "profile credential")
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
		OutputSchema: map[string]any{
			"type": "object", "required": []string{"data", "cursor"},
			"properties": map[string]any{
				"data":   map[string]any{"type": "array", "items": map[string]string{"type": "object"}},
				"cursor": map[string]string{"type": "object"},
			},
		},
	}, hints).ForCommand("phabricator task list").WithOutbound(destination, fields...)
}

const wikimediaPublicSpace = "PHID-SPCE-6l6g5p53yi3mypnlpxjw"

// Preserve the MCP disclosure scope for tasks whose public visibility cannot
// be verified, even when the selected account can read additional fields.
func limitedProfileTaskSearch(client *conduitClient) taskSearch {
	wikimedia := strings.TrimRight(client.baseURL, "/") == defaultPhabricatorURL
	return func(ctx context.Context, opts searchOptions) (taskPage, error) {
		page, err := client.searchTasks(ctx, opts)
		if err != nil {
			return page, err
		}
		for i, task := range page.Data {
			if wikimedia && task.Fields.SpacePHID == wikimediaPublicSpace && task.Fields.Policy["view"] == "public" {
				continue
			}
			page.Cursor.Before = nil
			summary := phabTask{ID: task.ID}
			summary.Fields.Name = task.Fields.Name
			summary.Fields.Status.Name = task.Fields.Status.Name
			summary.Fields.Priority.Name = task.Fields.Priority.Name
			page.Data[i] = summary
		}
		return page, nil
	}
}
