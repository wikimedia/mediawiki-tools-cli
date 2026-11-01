package wikiapi

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Page is the common identity returned by all read-only page selectors.
type Page struct {
	ID        int64  `json:"pageid"`
	Namespace int    `json:"ns"`
	Title     string `json:"title"`
	Missing   bool   `json:"missing,omitempty"`
	Invalid   bool   `json:"invalid,omitempty"`
}

// PageSelection represents exactly one page source. Empty Source selects allpages.
type PageSelection struct {
	Source    string
	Value     string
	Namespace *int
	Limit     int
	Grep      string
	// AllPages holds the existing allpages-only filters using API parameter names.
	AllPages url.Values
	Titles   []string
}

// ReadTitles accepts one plain title per line, skips blanks and deduplicates in
// input order. It deliberately does not interpret wikitext or execute scripts.
func ReadTitles(input io.Reader) ([]string, error) {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	titles := []string{}
	seen := map[string]bool{}
	for scanner.Scan() {
		title := strings.TrimSpace(scanner.Text())
		if title != "" && !seen[title] {
			seen[title] = true
			titles = append(titles, title)
		}
	}
	return titles, scanner.Err()
}

// SelectPages follows the complete continuation map and counts distinct matching
// pages toward Limit, regardless of server page sizes or duplicate responses.
func (c *Client) SelectPages(ctx context.Context, options PageSelection) ([]Page, error) {
	if options.Limit < 0 {
		return nil, fmt.Errorf("--limit must be nonnegative")
	}
	pages := []Page{}
	seen := map[string]bool{}
	accept := func(batch []Page) bool {
		for _, page := range batch {
			if page.Missing || page.Invalid || (options.Namespace != nil && page.Namespace != *options.Namespace) || (options.Grep != "" && !strings.Contains(page.Title, options.Grep)) {
				continue
			}
			key := page.Title
			if page.ID != 0 {
				key = strconv.FormatInt(page.ID, 10)
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			pages = append(pages, page)
			if options.Limit > 0 && len(pages) >= options.Limit {
				return true
			}
		}
		return false
	}
	if options.Source == "titles" {
		for offset := 0; offset < len(options.Titles); offset += 50 {
			end := min(offset+50, len(options.Titles))
			data, err := c.Action(ctx, http.MethodGet, url.Values{"action": {"query"}, "titles": {apiList(options.Titles[offset:end])}, "prop": {"info"}})
			if err != nil {
				return nil, err
			}
			var result struct {
				Query struct {
					Pages      []Page `json:"pages"`
					Normalized []struct {
						From string `json:"from"`
						To   string `json:"to"`
					} `json:"normalized"`
				} `json:"query"`
			}
			if err = json.Unmarshal(data, &result); err != nil {
				return nil, err
			}
			// API may reorder pages and normalize titles. Preserve file order where
			// possible, applying the API's normalization before matching results.
			byTitle := map[string]Page{}
			for _, page := range result.Query.Pages {
				byTitle[page.Title] = page
			}
			normalized := map[string]string{}
			for _, title := range result.Query.Normalized {
				normalized[title.From] = title.To
			}
			batch := []Page{}
			for _, title := range options.Titles[offset:end] {
				if target, ok := normalized[title]; ok {
					title = target
				}
				if page, ok := byTitle[title]; ok {
					batch = append(batch, page)
				}
			}
			if accept(batch) {
				return pages, nil
			}
		}
		return pages, nil
	}
	fields := url.Values{"action": {"query"}}
	module, prefix := "allpages", "ap"
	switch options.Source {
	case "", "allpages":
		for key, values := range options.AllPages {
			fields[key] = append([]string(nil), values...)
		}
	case "search":
		module, prefix = "search", "sr"
		fields.Set("srsearch", options.Value)
		fields.Set("srprop", "")
	case "category":
		module, prefix = "categorymembers", "cm"
		title := options.Value
		if !strings.Contains(title, ":") {
			title = "Category:" + title
		}
		fields.Set("cmtitle", title)
	case "backlinks":
		module, prefix = "backlinks", "bl"
		fields.Set("bltitle", options.Value)
	case "transclusions":
		module, prefix = "embeddedin", "ei"
		fields.Set("eititle", options.Value)
	default:
		return nil, fmt.Errorf("unsupported page selector %q", options.Source)
	}
	fields.Set("list", module)
	fields.Set(prefix+"limit", "max")
	if options.Namespace != nil {
		fields.Set(prefix+"namespace", strconv.Itoa(*options.Namespace))
	}
	previous := map[string]bool{}
	previousKeys := []string{}
	for {
		data, err := c.Action(ctx, http.MethodGet, fields)
		if err != nil {
			return nil, err
		}
		var result struct {
			Query    map[string]json.RawMessage `json:"query"`
			Continue url.Values                 `json:"-"`
		}
		if err = json.Unmarshal(data, &result); err != nil {
			return nil, err
		}
		raw, ok := result.Query[module]
		if !ok {
			return nil, fmt.Errorf("wiki response is missing query.%s", module)
		}
		var batch []Page
		if err = json.Unmarshal(raw, &batch); err != nil {
			return nil, err
		}
		if accept(batch) {
			return pages, nil
		}
		var envelope struct {
			Continue map[string]json.RawMessage `json:"continue"`
		}
		if err = json.Unmarshal(data, &envelope); err != nil {
			return nil, err
		}
		if len(envelope.Continue) == 0 {
			return pages, nil
		}
		continuation := make(url.Values)
		for key, raw := range envelope.Continue {
			var value string
			if json.Unmarshal(raw, &value) != nil {
				value = string(raw)
			}
			continuation.Set(key, value)
		}
		marker := continuation.Encode()
		if previous[marker] {
			return nil, fmt.Errorf("wiki repeated its continuation cursor")
		}
		previous[marker] = true
		for _, key := range previousKeys {
			fields.Del(key)
		}
		previousKeys = nil
		for key, values := range continuation {
			fields[key] = values
			previousKeys = append(previousKeys, key)
		}
	}
}

func apiList(values []string) string {
	separator := "|"
	for _, value := range values {
		if strings.Contains(value, "|") {
			separator = "\x1f"
			break
		}
	}
	if separator == "\x1f" {
		return separator + strings.Join(values, separator)
	}
	return strings.Join(values, separator)
}
