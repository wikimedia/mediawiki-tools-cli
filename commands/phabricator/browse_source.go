package phabricator

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"gitlab.wikimedia.org/repos/releng/cli/internal/util/browser"
)

func browseCursor(cursor *string) string {
	if cursor == nil {
		return ""
	}
	return *cursor
}

func newBrowseSource(client *conduitClient) browseSource {
	base := strings.TrimRight(client.baseURL, "/")
	return browseSource{
		projects: func(ctx context.Context, query, after, before string) (browsePage, error) {
			page, err := client.searchProjects(ctx, query, 25, after, before)
			if err != nil {
				return browsePage{}, err
			}
			result := browsePage{after: browseCursor(page.Cursor.After), before: browseCursor(page.Cursor.Before)}
			for _, project := range page.Data {
				result.items = append(result.items, browseItem{key: project.PHID, title: project.Fields.Name, subtitle: project.PHID, url: fmt.Sprintf("%s/project/profile/%d/", base, project.ID)})
			}
			return result, nil
		},
		columns: func(ctx context.Context, project string) ([]browseColumn, error) {
			page, err := client.listColumns(ctx, project)
			if err != nil {
				return nil, err
			}
			result := make([]browseColumn, 0, len(page.Data))
			for _, column := range page.Data {
				result = append(result, browseColumn{name: column.Fields.Name, phid: column.PHID})
			}
			return result, nil
		},
		tasks: func(ctx context.Context, query browseQuery) (browsePage, error) {
			opts := searchOptions{Project: query.project, Column: query.column, Search: query.search, Limit: 25, After: query.after, Before: query.before}
			if !query.allStatuses {
				opts.Statuses = []string{"open()"}
			}
			page, err := client.searchTasks(ctx, opts)
			if err != nil {
				return browsePage{}, err
			}
			result := browsePage{after: browseCursor(page.Cursor.After), before: browseCursor(page.Cursor.Before)}
			for _, task := range page.Data {
				ref := fmt.Sprintf("T%d", task.ID)
				result.items = append(result.items, browseItem{key: ref, title: ref + "  " + task.Fields.Name, subtitle: task.Fields.Status.Name + " · " + task.Fields.Priority.Name, url: base + "/" + ref})
			}
			return result, nil
		},
		detail: func(ctx context.Context, ref string) (string, error) {
			task, err := client.getTaskContext(ctx, ref)
			if err != nil {
				return "", err
			}
			transactions, err := client.taskTransactions(ctx, task.PHID)
			if err != nil {
				return "", err
			}
			authors := []string{task.Fields.AuthorPHID}
			for _, transaction := range transactions {
				authors = append(authors, transaction.AuthorPHID)
			}
			users, err := client.getUsersContext(ctx, authors)
			if err != nil {
				return "", err
			}
			name := func(phid string) string {
				if user := users[phid]; user[0] != "" {
					return user[0]
				}
				return phid
			}
			sort.SliceStable(transactions, func(i, j int) bool { return transactions[i].ID < transactions[j].ID })
			var b strings.Builder
			fmt.Fprintf(&b, "%s/%s\n%s · %s · by %s\n\n%s\n", base, ref, task.Fields.Status.Name, task.Fields.Priority.Name, name(task.Fields.AuthorPHID), task.Fields.Description.Raw)
			for _, transaction := range transactions {
				for _, comment := range transaction.Comments {
					if !comment.Removed && comment.Content.Raw != "" {
						fmt.Fprintf(&b, "\n── %s · %s ──\n%s\n", name(transaction.AuthorPHID), formatDate(comment.DateCreated), comment.Content.Raw)
					}
				}
			}
			return b.String(), nil
		},
		open: browser.OpenURL,
	}
}
