package wikiapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func FileTitle(file string) (string, error) {
	if strings.TrimSpace(file) == "" {
		return "", fmt.Errorf("file title cannot be empty")
	}
	if strings.Contains(file, "://") {
		return "", fmt.Errorf("use a file title, not an asset URL")
	}
	if strings.Contains(file, ":") {
		return file, nil
	}
	return "File:" + file, nil
}

// File metadata may come from a shared repository even when its local file page
// is missing. Asset URLs are returned as data, never fetched with wiki credentials.
func (c *Client) fileProperty(ctx context.Context, file string, fields url.Values, requireInfo bool) (map[string]json.RawMessage, map[string]json.RawMessage, error) {
	title, err := FileTitle(file)
	if err != nil {
		return nil, nil, err
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
	if json.Unmarshal(data, &result) != nil || len(result.Query.Pages) != 1 {
		return nil, nil, fmt.Errorf("wiki did not return file metadata")
	}
	page := result.Query.Pages[0]
	if _, invalid := page["invalid"]; invalid {
		return nil, nil, fmt.Errorf("invalid file title")
	}
	var info []json.RawMessage
	if requireInfo && (json.Unmarshal(page["imageinfo"], &info) != nil || len(info) == 0) {
		return nil, nil, fmt.Errorf("file %q does not exist or has no readable file information", title)
	}
	return page, result.Continue, nil
}

func (c *Client) FileInfo(ctx context.Context, file string, extended bool) (map[string]json.RawMessage, error) {
	properties := "timestamp|user|userid|size|sha1|mime|mediatype|url|canonicaltitle"
	if extended {
		properties += "|metadata|commonmetadata|extmetadata"
	}
	page, _, err := c.fileProperty(ctx, file, url.Values{"prop": {"imageinfo"}, "iiprop": {properties}, "iilimit": {"1"}}, true)
	return page, err
}

func (c *Client) FileUsage(ctx context.Context, file string, namespace *int, limit int) ([]Page, error) {
	if limit < 0 {
		return nil, fmt.Errorf("--limit must be nonnegative")
	}
	// Verify existence separately: following imageinfo continuation here would
	// accidentally walk upload history while paginating usage.
	if _, _, err := c.fileProperty(ctx, file, url.Values{"prop": {"imageinfo"}, "iiprop": {"size"}, "iilimit": {"1"}}, true); err != nil {
		return nil, err
	}
	fields := url.Values{"prop": {"fileusage"}, "fuprop": {"pageid|title|redirect"}, "fulimit": {"max"}}
	if limit > 0 {
		fields.Set("fulimit", strconv.Itoa(min(limit, 500)))
	}
	if namespace != nil {
		fields.Set("funamespace", strconv.Itoa(*namespace))
	}
	pages := []Page{}
	seen := map[string]bool{}
	continuation := queryContinuation{}
	for {
		page, next, err := c.fileProperty(ctx, file, fields, false)
		if err != nil {
			return nil, err
		}
		var batch []Page
		if raw, exists := page["fileusage"]; exists {
			if err = json.Unmarshal(raw, &batch); err != nil {
				return nil, err
			}
		}
		for _, used := range batch {
			if namespace != nil && used.Namespace != *namespace {
				continue
			}
			key := used.Title
			if used.ID > 0 {
				key = strconv.FormatInt(used.ID, 10)
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			pages = append(pages, used)
			if limit > 0 && len(pages) >= limit {
				return pages, nil
			}
		}
		more, err := continuation.advance(fields, next)
		if err != nil {
			return nil, err
		}
		if !more {
			return pages, nil
		}
	}
}
