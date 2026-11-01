package wikiapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// pageProperty resolves exactly one local page without following redirects by
// default. It retains native info fields and the API's opaque continuation map.
func (c *Client) pageProperty(ctx context.Context, title string, fields url.Values) (map[string]json.RawMessage, map[string]json.RawMessage, error) {
	if strings.TrimSpace(title) == "" {
		return nil, nil, fmt.Errorf("page title cannot be empty")
	}
	fields = cloneValues(fields)
	fields.Set("action", "query")
	fields.Set("titles", apiList([]string{title}))
	data, err := c.Action(ctx, http.MethodGet, fields)
	if err != nil {
		return nil, nil, err
	}
	var result struct {
		Query struct {
			Pages []map[string]json.RawMessage `json:"pages"`
		} `json:"query"`
		Continue map[string]json.RawMessage `json:"continue"`
	}
	if err = json.Unmarshal(data, &result); err != nil {
		return nil, nil, err
	}
	if len(result.Query.Pages) != 1 {
		return nil, nil, fmt.Errorf("wiki did not return page %q", title)
	}
	page := result.Query.Pages[0]
	if _, invalid := page["invalid"]; invalid {
		return nil, nil, fmt.Errorf("invalid page title %q", title)
	}
	if _, missing := page["missing"]; missing {
		return nil, nil, fmt.Errorf("page %q does not exist", title)
	}
	if rawString(page["title"]) == "" {
		return nil, nil, fmt.Errorf("wiki returned page metadata without a title")
	}
	return page, result.Continue, nil
}

// PageInfo returns native page identity, URL and protection metadata.
func (c *Client) PageInfo(ctx context.Context, title string, followRedirects bool) (map[string]json.RawMessage, error) {
	fields := url.Values{"prop": {"info"}, "inprop": {"url|protection|displaytitle"}}
	if followRedirects {
		fields.Set("redirects", "1")
	}
	page, _, err := c.pageProperty(ctx, title, fields)
	return page, err
}

type Revision struct {
	ID            int64    `json:"revid"`
	ParentID      int64    `json:"parentid"`
	Timestamp     string   `json:"timestamp"`
	User          string   `json:"user,omitempty"`
	UserID        int64    `json:"userid,omitempty"`
	UserHidden    bool     `json:"userhidden,omitempty"`
	Comment       string   `json:"comment,omitempty"`
	CommentHidden bool     `json:"commenthidden,omitempty"`
	Size          int64    `json:"size"`
	Minor         bool     `json:"minor,omitempty"`
	Anonymous     bool     `json:"anon,omitempty"`
	Suppressed    bool     `json:"suppressed,omitempty"`
	Tags          []string `json:"tags,omitempty"`
}
type HistoryOptions struct {
	Limit                         int
	Order                         string
	Start, End, User, ExcludeUser string
}

// HistoryTimestamp normalizes an explicitly supplied RFC3339 timestamp for the
// Action API. Empty bounds are omitted rather than filled with the local time.
func HistoryTimestamp(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return "", fmt.Errorf("timestamp %q must use RFC3339", value)
	}
	return parsed.UTC().Format("2006-01-02T15:04:05Z"), nil
}

func (options HistoryOptions) Validate() error {
	if options.Limit < 0 {
		return fmt.Errorf("--limit must be nonnegative")
	}
	if options.Order != "older" && options.Order != "newer" {
		return fmt.Errorf("--order must be older or newer")
	}
	if options.User != "" && options.ExcludeUser != "" {
		return fmt.Errorf("choose either --by-user or --exclude-user")
	}
	if _, err := HistoryTimestamp(options.Start); err != nil {
		return err
	}
	if _, err := HistoryTimestamp(options.End); err != nil {
		return err
	}
	return nil
}

func (c *Client) PageHistory(ctx context.Context, title string, options HistoryOptions) ([]Revision, error) {
	if options.Order == "" {
		options.Order = "older"
	}
	if err := options.Validate(); err != nil {
		return nil, err
	}
	fields := url.Values{"prop": {"revisions"}, "rvprop": {"ids|timestamp|user|userid|size|comment|flags|tags"}, "rvdir": {options.Order}, "rvlimit": {"max"}}
	if options.Limit > 0 {
		fields.Set("rvlimit", strconv.Itoa(min(options.Limit, 500)))
	}
	start, _ := HistoryTimestamp(options.Start)
	end, _ := HistoryTimestamp(options.End)
	for key, value := range map[string]string{"rvstart": start, "rvend": end, "rvuser": options.User, "rvexcludeuser": options.ExcludeUser} {
		if value != "" {
			fields.Set(key, value)
		}
	}
	revisions := []Revision{}
	seen := map[int64]bool{}
	continuation := queryContinuation{}
	for {
		page, next, err := c.pageProperty(ctx, title, fields)
		if err != nil {
			return nil, err
		}
		var batch []Revision
		if raw, ok := page["revisions"]; ok {
			if err = json.Unmarshal(raw, &batch); err != nil {
				return nil, err
			}
		}
		for _, revision := range batch {
			if revision.ID <= 0 {
				return nil, fmt.Errorf("wiki returned a revision without an id")
			}
			if seen[revision.ID] {
				continue
			}
			seen[revision.ID] = true
			revisions = append(revisions, revision)
			if options.Limit > 0 && len(revisions) >= options.Limit {
				return revisions, nil
			}
		}
		more, err := continuation.advance(fields, next)
		if err != nil {
			return nil, err
		}
		if !more {
			return revisions, nil
		}
	}
}

type PageCategory struct {
	Namespace     int    `json:"ns"`
	Title         string `json:"title"`
	Hidden        bool   `json:"hidden,omitempty"`
	SortKey       string `json:"sortkey,omitempty"`
	SortKeyPrefix string `json:"sortkeyprefix,omitempty"`
}

func ValidateCategoryFilter(hidden string, limit int) error {
	if limit < 0 {
		return fmt.Errorf("--limit must be nonnegative")
	}
	switch hidden {
	case "all", "only", "exclude":
		return nil
	}
	return fmt.Errorf("--hidden must be all, only or exclude")
}

// PageCategories returns categories attached to the given page. This differs
// from SelectPages' category-members source; attached-category reads are not recursive.
func (c *Client) PageCategories(ctx context.Context, title, hidden string, limit int) ([]PageCategory, error) {
	if err := ValidateCategoryFilter(hidden, limit); err != nil {
		return nil, err
	}
	fields := url.Values{"prop": {"categories"}, "clprop": {"hidden|sortkey"}, "cllimit": {"max"}}
	if limit > 0 {
		fields.Set("cllimit", strconv.Itoa(min(limit, 500)))
	}
	if hidden == "only" {
		fields.Set("clshow", "hidden")
	} else if hidden == "exclude" {
		fields.Set("clshow", "!hidden")
	}
	categories := []PageCategory{}
	seen := map[string]bool{}
	continuation := queryContinuation{}
	for {
		page, next, err := c.pageProperty(ctx, title, fields)
		if err != nil {
			return nil, err
		}
		var batch []PageCategory
		if raw, ok := page["categories"]; ok {
			if err = json.Unmarshal(raw, &batch); err != nil {
				return nil, err
			}
		}
		for _, category := range batch {
			if (hidden == "only" && !category.Hidden) || (hidden == "exclude" && category.Hidden) {
				continue
			}
			if category.Title == "" {
				return nil, fmt.Errorf("wiki returned a category without a title")
			}
			if seen[category.Title] {
				continue
			}
			seen[category.Title] = true
			categories = append(categories, category)
			if limit > 0 && len(categories) >= limit {
				return categories, nil
			}
		}
		more, err := continuation.advance(fields, next)
		if err != nil {
			return nil, err
		}
		if !more {
			return categories, nil
		}
	}
}
