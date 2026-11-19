package phabricator

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type pageCursor struct {
	After  *string `json:"after"`
	Before *string `json:"before"`
	Limit  int     `json:"limit"`
}

type taskPage struct {
	Data   []phabTask `json:"data"`
	Cursor pageCursor `json:"cursor"`
}
type projectPage struct {
	Data   []phabProject `json:"data"`
	Cursor pageCursor    `json:"cursor"`
}
type columnPage struct {
	Data   []phabColumn `json:"data"`
	Cursor pageCursor   `json:"cursor"`
}

type searchOptions struct {
	Search     string   `json:"query,omitempty" jsonschema:"Full-text search sent to Phabricator. Use public terms for anonymous searches."`
	Projects   []string `json:"projects,omitempty" jsonschema:"Project tags, exact names, numeric IDs, or PHIDs. Tasks must match every project."`
	Column     string   `json:"column,omitempty" jsonschema:"Exact workboard column name or PHID. Requires one project and a visible workboard."`
	AssignedTo string   `json:"assignedTo,omitempty" jsonschema:"Assignee username, or me when a profile is bound."`
	Author     string   `json:"author,omitempty" jsonschema:"Author username, or me when a profile is bound."`
	Order      string   `json:"order,omitempty" jsonschema:"updated (default) or newest (most recently created first)."`
	Statuses   []string `json:"statuses,omitempty" jsonschema:"Task status keys. Defaults to open(); use all to include closed tasks."`
	Priorities []int    `json:"priorities,omitempty" jsonschema:"Numeric priorities, for example 100 (Unbreak Now) and 80 (High)."`
	Unassigned bool     `json:"unassigned,omitempty" jsonschema:"Only return tasks without an assignee. Mutually exclusive with assignedTo."`
	After      string   `json:"after,omitempty" jsonschema:"The cursor.after value returned by the previous page."`
	Before     string   `json:"before,omitempty" jsonschema:"The cursor.before value returned by the current page."`
	Limit      int      `json:"-"`
	All        bool     `json:"-"`
}

const defaultSearchPageLimit = 50

func defaultTaskListOptions() searchOptions {
	return searchOptions{Limit: defaultSearchPageLimit, Order: "updated", Statuses: []string{"open()"}}
}

func (opts searchOptions) params() (map[string]interface{}, error) {
	params, err := queryParams(opts.Limit, opts.After, opts.Before)
	if err != nil {
		return nil, err
	}
	if err := validateTaskOrder(opts.Order); err != nil {
		return nil, err
	}
	if opts.Column != "" && len(opts.Projects) != 1 {
		return nil, fmt.Errorf("--column requires exactly one --project")
	}
	for _, project := range opts.Projects {
		if strings.TrimPrefix(strings.TrimSpace(project), "#") == "" {
			return nil, fmt.Errorf("project is required")
		}
	}
	if opts.All && (opts.After != "" || opts.Before != "") {
		return nil, fmt.Errorf("--all cannot be combined with a cursor")
	}
	if opts.Unassigned && opts.AssignedTo != "" {
		return nil, fmt.Errorf("--unassigned cannot be combined with --assigned-to")
	}
	for _, username := range []string{opts.Author, opts.AssignedTo} {
		if username != "" && (strings.TrimSpace(username) == "" || strings.ContainsAny(username, ",()")) {
			return nil, fmt.Errorf("provide one Phabricator username per author or assignee filter")
		}
	}
	return params, nil
}

func findSearchColumn(columns []phabColumn, ref string) (string, error) {
	var matches []string
	for _, column := range columns {
		if column.PHID == ref || strings.EqualFold(column.Fields.Name, ref) {
			matches = append(matches, column.PHID)
		}
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("column %q did not resolve uniquely; use its exact name or PHID", ref)
	}
	return matches[0], nil
}

func validateTaskOrder(order string) error {
	if order != "" && order != "updated" && order != "newest" {
		return fmt.Errorf("--order must be updated or newest")
	}
	return nil
}

func queryParams(limit int, after, before string) (map[string]interface{}, error) {
	if limit < 1 || limit > 100 {
		return nil, fmt.Errorf("--limit must be between 1 and 100")
	}
	if after != "" && before != "" {
		return nil, fmt.Errorf("--after and --before are mutually exclusive")
	}
	params := map[string]interface{}{"limit": limit}
	if after != "" {
		params["after"] = after
	}
	if before != "" {
		params["before"] = before
	}
	return params, nil
}

// exactProject accepts a hashtag slug, numeric ID, PHID, or an exact display name.
// Search results are never silently interpreted as the first fuzzy match.
func (c *conduitClient) exactProject(ctx context.Context, ref string) (*phabProject, error) {
	ref = strings.TrimPrefix(strings.TrimSpace(ref), "#")
	if ref == "" {
		return nil, fmt.Errorf("project is required")
	}
	constraints := map[string]interface{}{"slugs": []string{ref}}
	if strings.HasPrefix(ref, "PHID-PROJ-") {
		constraints = map[string]interface{}{"phids": []string{ref}}
	} else if id, err := strconv.Atoi(ref); err == nil {
		constraints = map[string]interface{}{"ids": []int{id}}
	}
	raw, err := c.postContext(ctx, "project.search", map[string]interface{}{"constraints": constraints, "limit": 2})
	if err != nil {
		return nil, err
	}
	var page projectPage
	if err = json.Unmarshal(raw, &page); err != nil {
		return nil, err
	}
	if len(page.Data) == 0 && !strings.HasPrefix(ref, "PHID-") {
		// A display name can contain spaces; exact matches must still be unique.
		after := ""
		for {
			candidates, e := c.searchProjects(ctx, ref, 100, after, "")
			if e != nil {
				return nil, e
			}
			for _, p := range candidates.Data {
				if strings.EqualFold(p.Fields.Name, ref) {
					page.Data = append(page.Data, p)
				}
			}
			if candidates.Cursor.After == nil {
				break
			}
			if *candidates.Cursor.After == after {
				return nil, fmt.Errorf("project search repeated its cursor")
			}
			after = *candidates.Cursor.After
		}
	}
	if len(page.Data) != 1 {
		return nil, fmt.Errorf("project %q did not resolve to exactly one project; use its hashtag or PHID", ref)
	}
	return &page.Data[0], nil
}

func (c *conduitClient) searchProjects(ctx context.Context, query string, limit int, after, before string) (projectPage, error) {
	page := projectPage{Data: []phabProject{}}
	params, err := queryParams(limit, after, before)
	if err != nil {
		return page, err
	}
	params["constraints"] = map[string]interface{}{"query": query}
	raw, err := c.postContext(ctx, "project.search", params)
	if err != nil {
		return page, err
	}
	err = json.Unmarshal(raw, &page)
	return page, err
}

func (c *conduitClient) listColumns(ctx context.Context, project string) (columnPage, error) {
	p, err := c.exactProject(ctx, project)
	if err != nil {
		return columnPage{Data: []phabColumn{}}, err
	}
	return c.listProjectColumns(ctx, p.PHID)
}

func (c *conduitClient) listProjectColumns(ctx context.Context, phid string) (columnPage, error) {
	page := columnPage{Data: []phabColumn{}}
	after := ""
	for {
		params := map[string]interface{}{"constraints": map[string]interface{}{"projects": []string{phid}, "statuses": []string{"0"}}, "limit": 100}
		if after != "" {
			params["after"] = after
		}
		raw, e := c.postContext(ctx, "project.column.search", params)
		if e != nil {
			return page, e
		}
		var next columnPage
		if e = json.Unmarshal(raw, &next); e != nil {
			return page, e
		}
		page.Data = append(page.Data, next.Data...)
		if next.Cursor.After == nil {
			break
		}
		if *next.Cursor.After == after {
			return page, fmt.Errorf("column search repeated its cursor")
		}
		after = *next.Cursor.After
	}
	return page, nil
}

func (c *conduitClient) searchTasks(ctx context.Context, opts searchOptions) (taskPage, error) {
	page := taskPage{Data: []phabTask{}}
	params, err := opts.params()
	if err != nil {
		return page, err
	}
	constraints := map[string]interface{}{}
	if len(opts.Statuses) > 0 && !(len(opts.Statuses) == 1 && opts.Statuses[0] == "all") {
		constraints["statuses"] = opts.Statuses
	}
	if opts.Search != "" {
		constraints["query"] = opts.Search
	}
	projects := make([]string, 0, len(opts.Projects))
	if len(opts.Projects) > 0 {
		for _, ref := range opts.Projects {
			project, err := c.exactProject(ctx, ref)
			if err != nil {
				return page, err
			}
			projects = append(projects, project.PHID)
		}
		constraints["projects"] = projects
	}
	if opts.Column != "" {
		columns, err := c.listProjectColumns(ctx, projects[0])
		if err != nil {
			return page, err
		}
		column, err := findSearchColumn(columns.Data, opts.Column)
		if err != nil {
			return page, err
		}
		constraints["columnPHIDs"] = []string{column}
	}
	if len(opts.Priorities) > 0 {
		constraints["priorities"] = opts.Priorities
	}
	if opts.AssignedTo != "" {
		user, e := c.resolveUserContext(ctx, opts.AssignedTo)
		if e != nil {
			return page, e
		}
		constraints["assigned"] = []string{user.PHID}
	}
	if opts.Unassigned {
		constraints["assigned"] = []string{"none()"}
	}
	if opts.Author != "" {
		user, err := c.resolveUserContext(ctx, opts.Author)
		if err != nil {
			return page, err
		}
		constraints["authorPHIDs"] = []string{user.PHID}
	}
	if opts.Order != "" {
		params["order"] = opts.Order
	}
	params["constraints"] = constraints
	return c.searchTaskPages(ctx, params, opts.All)
}

// searchTaskPages shares paging and attachments between resolved board columns
// and user-supplied task searches.
func (c *conduitClient) searchTaskPages(ctx context.Context, params map[string]interface{}, all bool) (taskPage, error) {
	page := taskPage{Data: []phabTask{}}
	if _, exists := params["order"]; !exists {
		params["order"] = "updated"
	}
	params["attachments"] = map[string]interface{}{"projects": true, "columns": true}
	seen := map[string]bool{}
	for {
		raw, e := c.postContext(ctx, "maniphest.search", params)
		if e != nil {
			return page, e
		}
		var next taskPage
		if e = json.Unmarshal(raw, &next); e != nil {
			return page, e
		}
		for _, task := range next.Data {
			page.Data = append(page.Data, c.taskForList(task))
		}
		page.Cursor = next.Cursor
		if !all || next.Cursor.After == nil {
			break
		}
		after := *next.Cursor.After
		if seen[after] {
			return page, fmt.Errorf("task search repeated its cursor")
		}
		seen[after] = true
		params["after"] = after
	}
	return page, nil
}

func (c *conduitClient) taskTransactions(ctx context.Context, phid string) ([]phabTransaction, error) {
	if c.access.publicOnly {
		if _, err := c.getTaskByPHIDContext(ctx, phid); err != nil {
			return nil, err
		}
	}
	txs := []phabTransaction{}
	after := ""
	seen := map[string]bool{}
	for {
		params := map[string]interface{}{"objectIdentifier": phid, "limit": 100}
		if after != "" {
			params["after"] = after
		}
		raw, err := c.postContext(ctx, "transaction.search", params)
		if err != nil {
			return nil, err
		}
		var page struct {
			Data   []phabTransaction `json:"data"`
			Cursor pageCursor        `json:"cursor"`
		}
		if err = json.Unmarshal(raw, &page); err != nil {
			return nil, err
		}
		txs = append(txs, page.Data...)
		if page.Cursor.After == nil {
			return txs, nil
		}
		after = *page.Cursor.After
		if seen[after] {
			return nil, fmt.Errorf("transaction search repeated its cursor")
		}
		seen[after] = true
	}
}

func (c *conduitClient) getTaskContext(ctx context.Context, ref string) (*phabTask, error) {
	id, err := strconv.Atoi(strings.TrimPrefix(strings.ToUpper(ref), "T"))
	if err != nil || id <= 0 {
		return nil, fmt.Errorf("invalid task reference %q", ref)
	}
	raw, err := c.postContext(ctx, "maniphest.search", map[string]interface{}{"constraints": map[string]interface{}{"ids": []int{id}}, "attachments": map[string]interface{}{"projects": true, "columns": true}})
	if err != nil {
		return nil, err
	}
	var page taskPage
	if err = json.Unmarshal(raw, &page); err != nil {
		return nil, err
	}
	if len(page.Data) != 1 {
		return nil, fmt.Errorf("task T%d not found", id)
	}
	if err := c.requirePublicTask(page.Data[0]); err != nil {
		return nil, err
	}
	return &page.Data[0], nil
}
