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
	Project, Column, Search, AssignedTo string
	Statuses                            []string
	Limit                               int
	After, Before                       string
	All                                 bool
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
	page := columnPage{Data: []phabColumn{}}
	p, err := c.exactProject(ctx, project)
	if err != nil {
		return page, err
	}
	after := ""
	for {
		params := map[string]interface{}{"constraints": map[string]interface{}{"projects": []string{p.PHID}, "statuses": []string{"0"}}, "limit": 100}
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
	params, err := queryParams(opts.Limit, opts.After, opts.Before)
	if err != nil {
		return page, err
	}
	if opts.All && (opts.After != "" || opts.Before != "") {
		return page, fmt.Errorf("--all cannot be combined with a cursor")
	}
	constraints := map[string]interface{}{}
	if len(opts.Statuses) > 0 && !(len(opts.Statuses) == 1 && opts.Statuses[0] == "all") {
		constraints["statuses"] = opts.Statuses
	}
	if opts.Search != "" {
		constraints["query"] = opts.Search
	}
	if opts.Column != "" && opts.Project == "" {
		return page, fmt.Errorf("--column requires --project")
	}
	if opts.Project != "" {
		project, e := c.exactProject(ctx, opts.Project)
		if e != nil {
			return page, e
		}
		constraints["projects"] = []string{project.PHID}
		if opts.Column != "" {
			columns, e := c.listColumns(ctx, project.PHID)
			if e != nil {
				return page, e
			}
			var matching []phabColumn
			for _, col := range columns.Data {
				if col.PHID == opts.Column || strings.EqualFold(col.Fields.Name, opts.Column) {
					matching = append(matching, col)
				}
			}
			if len(matching) != 1 {
				return page, fmt.Errorf("column %q did not resolve uniquely; use board columns to find its PHID", opts.Column)
			}
			constraints["columnPHIDs"] = []string{matching[0].PHID}
		}
	}
	if opts.AssignedTo != "" {
		user, e := c.resolveUserContext(ctx, opts.AssignedTo)
		if e != nil {
			return page, e
		}
		constraints["assigned"] = []string{user.PHID}
	}
	params["constraints"] = constraints
	return c.searchTaskPages(ctx, params, opts.All)
}

// searchTaskPages shares paging and attachments between resolved board columns
// and user-supplied task searches.
func (c *conduitClient) searchTaskPages(ctx context.Context, params map[string]interface{}, all bool) (taskPage, error) {
	page := taskPage{Data: []phabTask{}}
	params["order"] = "updated"
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
		page.Data = append(page.Data, next.Data...)
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
	return &page.Data[0], nil
}
