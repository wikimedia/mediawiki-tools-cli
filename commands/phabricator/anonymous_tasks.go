package phabricator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/spf13/cobra"
)

type anonymousComment struct {
	Author string
	Date   string
	Body   string
}

func (c anonymousPhabricator) task(ctx context.Context, ref string) (*phabTask, []anonymousComment, error) {
	doc, err := c.get(ctx, "/"+ref, nil)
	if err != nil {
		return nil, nil, err
	}
	title := doc.Find("meta[property='og:title']").AttrOr("content", "")
	name, found := strings.CutPrefix(title, ref+" ")
	if !found || strings.TrimSpace(name) == "" {
		return nil, nil, fmt.Errorf("Phabricator task %s was not found or is not publicly visible", ref)
	}
	id, _ := strconv.Atoi(ref[1:])
	task := &phabTask{ID: id}
	task.Fields.Name = name
	task.Fields.Description.Raw = anonymousRemarkupText(doc.Find(".phui-property-list-text-content .phabricator-remarkup").First())
	task.Fields.Status.Name = strings.TrimSpace(doc.Find(".phui-header-subheader .phui-tag-core").First().Text())
	events := doc.Find(".phui-timeline-event-view")
	if firstDate := events.Find(".phui-timeline-extra .print-only").First().Text(); firstDate != "" {
		if date, err := time.Parse("2006-01-02 15:04:05 (UTC+0)", strings.TrimSpace(firstDate)); err == nil {
			task.Fields.DateCreated = date.Unix()
		}
	}
	comments := []anonymousComment{}
	events.Has(".transaction-comment .phabricator-remarkup").Each(func(_ int, event *goquery.Selection) {
		comments = append(comments, anonymousComment{
			Author: strings.TrimSpace(event.Find(".phui-timeline-title .phui-link-person").First().Text()),
			Date:   strings.TrimSpace(event.Find(".phui-timeline-extra .print-only").First().Text()),
			Body:   anonymousRemarkupText(event.Find(".transaction-comment .phabricator-remarkup").First()),
		})
	})
	return task, comments, nil
}

func anonymousRemarkupText(remarkup *goquery.Selection) string {
	text := remarkup.Clone()
	text.Find("br").ReplaceWithHtml("\n")
	text.Find("p, li, pre, blockquote, h1, h2, h3, h4, h5, h6").AfterHtml("\n")
	return strings.TrimSpace(text.Text())
}

func printAnonymousTask(cmd *cobra.Command, task *phabTask) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "T%d — %s\n%s/T%d\n", task.ID, task.Fields.Name, defaultPhabricatorURL, task.ID)
	if task.Fields.Status.Name != "" {
		fmt.Fprintf(w, "Status: %s\n", task.Fields.Status.Name)
	}
	if task.Fields.Description.Raw != "" {
		fmt.Fprintf(w, "\n%s\n", task.Fields.Description.Raw)
	}
}

func printAnonymousComments(cmd *cobra.Command, comments []anonymousComment) {
	w := cmd.OutOrStdout()
	if len(comments) == 0 {
		fmt.Fprintln(w, "No public comments.")
	}
	for _, comment := range comments {
		fmt.Fprintf(w, "\n%s %s\n%s\n", comment.Author, comment.Date, comment.Body)
	}
}

// Public timelines have a different shape from Conduit transactions.
func readAnonymousTask(cmd *cobra.Command, args []string, commentsOnly bool) error {
	ref, err := requireTaskArg(args)
	if err != nil {
		return err
	}
	task, comments, err := newAnonymousPhabricator().task(cmd.Context(), ref)
	if err != nil {
		return err
	}
	if !commentsOnly {
		printAnonymousTask(cmd, task)
	}
	printAnonymousComments(cmd, comments)
	return nil
}

func (c anonymousPhabricator) searchTasks(ctx context.Context, opts searchOptions, priorities []string, unassigned bool) (taskPage, error) {
	page := taskPage{Data: []phabTask{}}
	if opts.Column != "" {
		return page, fmt.Errorf("--column requires a saved Phabricator profile")
	}
	if opts.AssignedTo == "me" || opts.Author == "me" {
		return page, fmt.Errorf("use an explicit username or select a Phabricator profile to search for me")
	}
	if unassigned && opts.AssignedTo != "" {
		return page, fmt.Errorf("--unassigned cannot be combined with --assigned-to")
	}
	for _, priority := range priorities {
		if _, err := strconv.Atoi(priority); err != nil {
			return page, fmt.Errorf("invalid priority %q: use a numeric Phabricator priority", priority)
		}
	}
	params := url.Values{"limit": {strconv.Itoa(opts.Limit)}, "order": {opts.Order}}
	if len(opts.Statuses) > 0 && !(len(opts.Statuses) == 1 && opts.Statuses[0] == "all") {
		params.Set("statuses", strings.Join(opts.Statuses, ","))
	}
	if opts.Project != "" {
		var projects []string
		for _, project := range strings.Split(opts.Project, ",") {
			project = strings.TrimSpace(project)
			if !projectPHIDPattern.MatchString(project) {
				resolved, err := c.project(ctx, project)
				if err != nil {
					return page, err
				}
				project = resolved.PHID
			}
			projects = append(projects, project)
		}
		params.Set("projects", strings.Join(projects, ","))
	}
	assigned := opts.AssignedTo
	if unassigned {
		assigned = "none()"
	}
	for name, value := range map[string]string{
		"query": opts.Search, "assigned": assigned, "authors": opts.Author,
		"priorities": strings.Join(priorities, ","), "after": opts.After, "before": opts.Before,
	} {
		if value != "" {
			params.Set(name, value)
		}
	}
	seen := map[string]bool{}
	for {
		doc, err := c.get(ctx, "/maniphest/", params)
		if err != nil {
			return page, err
		}
		tasks, err := parseAnonymousTaskList(doc)
		if err != nil {
			return page, err
		}
		page.Data = append(page.Data, tasks...)
		page.Cursor = anonymousPageCursor(doc, opts.Limit)
		if !opts.All || page.Cursor.After == nil {
			return page, nil
		}
		next := *page.Cursor.After
		if seen[next] {
			return page, fmt.Errorf("public Phabricator task search repeated its cursor")
		}
		seen[next] = true
		params.Set("after", next)
	}
}

func parseAnonymousTaskList(doc *goquery.Document) ([]phabTask, error) {
	items := doc.Find(".phui-oi-list-view .phui-oi")
	if items.Length() == 0 && strings.TrimSpace(doc.Find(".phui-info-view-body").First().Text()) != "No tasks found." {
		return nil, fmt.Errorf("unrecognized public Phabricator task search page")
	}

	// Phabricator stores each list item's status and priority in a tooltip.
	tips := map[string]string{}
	doc.Find("data[data-javelin-init-data]").Each(func(_ int, data *goquery.Selection) {
		var raw struct {
			Block int
			Data  []json.RawMessage
		}
		if json.Unmarshal([]byte(data.AttrOr("data-javelin-init-data", "")), &raw) != nil {
			return
		}
		for index, entry := range raw.Data {
			var meta struct{ Tip string }
			if json.Unmarshal(entry, &meta) == nil && meta.Tip != "" {
				tips[fmt.Sprintf("%d_%d", raw.Block, index)] = meta.Tip
			}
		}
	})

	tasks := []phabTask{}
	items.Each(func(_ int, item *goquery.Selection) {
		link := item.Find(".phui-oi-link").First()
		ref := strings.TrimPrefix(link.AttrOr("href", ""), "/")
		if !objectReferencePattern.MatchString(ref) || !strings.HasPrefix(ref, "T") {
			return
		}
		id, err := strconv.Atoi(ref[1:])
		if err != nil {
			return
		}
		task := phabTask{ID: id}
		task.Fields.Name = strings.TrimSpace(link.AttrOr("title", link.Text()))
		task.Fields.Name = strings.TrimPrefix(task.Fields.Name, ref+": ")
		status, priority, _ := strings.Cut(tips[item.Find(".phui-oi-status-icon").AttrOr("data-meta", "")], ", ")
		task.Fields.Status.Name = status
		task.Fields.Priority.Name = priority
		tasks = append(tasks, task)
	})
	if items.Length() > 0 && len(tasks) == 0 {
		return nil, fmt.Errorf("public Phabricator task search results did not contain recognizable tasks")
	}
	return tasks, nil
}
