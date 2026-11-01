package wikiapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type PageEditPlan struct {
	Title        string `json:"title"`
	Text         string `json:"text"`
	BaseRevision int64  `json:"baserevid"`
	CreateOnly   bool   `json:"createonly"`
}

func (c *Client) PlanPageEdit(ctx context.Context, title, text string, baseRevision int64, createOnly bool) (*PageEditPlan, error) {
	if strings.TrimSpace(title) == "" {
		return nil, fmt.Errorf("page title cannot be empty")
	}
	if (createOnly && baseRevision != 0) || (!createOnly && baseRevision <= 0) {
		return nil, fmt.Errorf("page updates require positive baserevid; new pages require createonly with baserevid 0")
	}
	data, err := c.Action(ctx, http.MethodGet, url.Values{"action": {"query"}, "prop": {"info"}, "titles": {apiList([]string{title})}})
	if err != nil {
		return nil, err
	}
	var result struct {
		Query struct {
			Pages []struct {
				Title    string `json:"title"`
				Missing  bool   `json:"missing"`
				Invalid  bool   `json:"invalid"`
				Revision int64  `json:"lastrevid"`
			} `json:"pages"`
		} `json:"query"`
	}
	if json.Unmarshal(data, &result) != nil || len(result.Query.Pages) != 1 {
		return nil, fmt.Errorf("wiki did not return a page edit precondition")
	}
	page := result.Query.Pages[0]
	if page.Invalid {
		return nil, fmt.Errorf("invalid page title")
	}
	if createOnly {
		if !page.Missing {
			return nil, fmt.Errorf("createonly page %q already exists", title)
		}
	} else {
		if page.Missing {
			return nil, fmt.Errorf("page %q does not exist", title)
		}
		if page.Revision != baseRevision {
			return nil, &ConflictError{Resource: "page " + title, Expected: baseRevision, Actual: page.Revision}
		}
	}
	return &PageEditPlan{Title: page.Title, Text: text, BaseRevision: baseRevision, CreateOnly: createOnly}, nil
}

func (c *Client) ExecutePageEdit(ctx context.Context, plan *PageEditPlan, options EditOptions) (json.RawMessage, error) {
	if plan == nil {
		return nil, fmt.Errorf("invalid page edit plan")
	}
	if options.BaseRevision > 0 && options.BaseRevision != plan.BaseRevision {
		return nil, &ConflictError{Resource: "page edit plan", Expected: options.BaseRevision, Actual: plan.BaseRevision}
	}
	fields := url.Values{"action": {"edit"}, "title": {plan.Title}, "text": {plan.Text}}
	if plan.CreateOnly {
		fields.Set("createonly", "1")
	}
	options.BaseRevision = plan.BaseRevision
	fields, err := c.editFields(options, fields)
	if err != nil {
		return nil, err
	}
	data, err := c.edit(ctx, fields)
	if err != nil {
		return nil, err
	}
	var result struct {
		Edit struct {
			Result string `json:"result"`
		} `json:"edit"`
	}
	if json.Unmarshal(data, &result) != nil || result.Edit.Result == "" {
		return nil, fmt.Errorf("page edit returned no recognizable result; inspect before retrying")
	}
	if result.Edit.Result != "Success" {
		detail := string(data)
		if len(detail) > 4096 {
			detail = detail[:4096]
		}
		return nil, &APIError{Code: "edit-failed", Info: detail}
	}
	return data, nil
}
