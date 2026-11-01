package wikiapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

type DiffSelector struct {
	Title    string
	Revision int64
}

func ParseDiffSelector(input string) (DiffSelector, error) {
	if strings.HasPrefix(input, "title:") {
		title := strings.TrimPrefix(input, "title:")
		if strings.TrimSpace(title) == "" {
			return DiffSelector{}, fmt.Errorf("title selector cannot be empty")
		}
		return DiffSelector{Title: title}, nil
	}
	explicit := strings.HasPrefix(input, "rev:")
	value := strings.TrimPrefix(input, "rev:")
	number, err := strconv.ParseInt(value, 10, 64)
	if explicit || err == nil {
		if err != nil || number <= 0 {
			return DiffSelector{}, fmt.Errorf("revision selector must be a positive integer")
		}
		return DiffSelector{Revision: number}, nil
	}
	if strings.TrimSpace(input) == "" {
		return DiffSelector{}, fmt.Errorf("diff selector cannot be empty")
	}
	return DiffSelector{Title: input}, nil
}

func setDiffSelector(fields url.Values, prefix string, selector DiffSelector) {
	if selector.Revision > 0 {
		fields.Set(prefix+"rev", strconv.FormatInt(selector.Revision, 10))
	} else {
		fields.Set(prefix+"title", selector.Title)
	}
}

// PageDiff compares explicit titles/revisions, or a single selector with its
// previous revision. No page content is downloaded separately or written.
func (c *Client) PageDiff(ctx context.Context, from, to string, unified bool) (map[string]json.RawMessage, error) {
	first, err := ParseDiffSelector(from)
	if err != nil {
		return nil, err
	}
	fields := url.Values{"action": {"compare"}, "prop": {"diff|ids|title|timestamp"}}
	if to == "" {
		previous, selected, err := c.diffRevisionPair(ctx, first)
		if err != nil {
			return nil, err
		}
		fields.Set("fromrev", strconv.FormatInt(previous, 10))
		fields.Set("torev", strconv.FormatInt(selected, 10))
	} else {
		setDiffSelector(fields, "from", first)
		second, e := ParseDiffSelector(to)
		if e != nil {
			return nil, e
		}
		setDiffSelector(fields, "to", second)
	}
	if unified {
		fields.Set("difftype", "unified")
	} else {
		fields.Set("difftype", "table")
	}
	data, err := c.Action(ctx, http.MethodGet, fields)
	var apiError *APIError
	if unified && errors.As(err, &apiError) && apiError.Code == "badvalue" && strings.Contains(apiError.Error(), "difftype") {
		fields.Set("difftype", "table")
		data, err = c.Action(ctx, http.MethodGet, fields)
	}
	if err != nil {
		return nil, err
	}
	var response struct {
		Compare map[string]json.RawMessage `json:"compare"`
	}
	if err = json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	if response.Compare == nil {
		return nil, fmt.Errorf("wiki did not return comparison data")
	}
	if _, ok := response.Compare["body"]; !ok {
		return nil, fmt.Errorf("wiki did not return diff body")
	}
	return response.Compare, nil
}

// diffRevisionPair reads identifiers only. Resolving both IDs before comparing
// also pins a title's selected revision if another edit arrives between reads.
func (c *Client) diffRevisionPair(ctx context.Context, selector DiffSelector) (int64, int64, error) {
	fields := url.Values{"action": {"query"}, "prop": {"revisions"}, "rvprop": {"ids"}}
	if selector.Revision > 0 {
		fields.Set("revids", strconv.FormatInt(selector.Revision, 10))
	} else {
		fields.Set("titles", apiList([]string{selector.Title}))
		fields.Set("rvlimit", "1")
	}
	data, err := c.Action(ctx, http.MethodGet, fields)
	if err != nil {
		return 0, 0, err
	}
	var result struct {
		Query struct {
			Pages []struct {
				Missing, Invalid bool
				Revisions        []struct {
					ID     int64  `json:"revid"`
					Parent *int64 `json:"parentid"`
				}
			} `json:"pages"`
		} `json:"query"`
	}
	if err = json.Unmarshal(data, &result); err != nil {
		return 0, 0, err
	}
	if len(result.Query.Pages) != 1 || result.Query.Pages[0].Missing || result.Query.Pages[0].Invalid || len(result.Query.Pages[0].Revisions) != 1 {
		return 0, 0, fmt.Errorf("selected page or revision is missing, deleted, or unavailable")
	}
	revision := result.Query.Pages[0].Revisions[0]
	if revision.ID <= 0 || selector.Revision > 0 && revision.ID != selector.Revision || revision.Parent == nil {
		return 0, 0, fmt.Errorf("wiki returned incomplete selected revision metadata")
	}
	if *revision.Parent == 0 {
		return 0, 0, fmt.Errorf("revision %d is the first revision and has no predecessor; specify two selectors to compare explicitly", revision.ID)
	}
	if *revision.Parent < 0 {
		return 0, 0, fmt.Errorf("wiki returned an invalid predecessor revision")
	}
	return *revision.Parent, revision.ID, nil
}

// DiffText extracts native unified text when available. Older table diffs are
// rendered as readable +/-/context lines, not advertised as applyable patches.
func DiffText(body string) (string, error) {
	if body == "" {
		return "", nil
	}
	root, err := html.Parse(strings.NewReader("<table>" + body + "</table>"))
	if err != nil {
		return "", err
	}
	text := func(node *html.Node) string {
		var b strings.Builder
		var walk func(*html.Node)
		walk = func(n *html.Node) {
			if n.Type == html.TextNode {
				b.WriteString(n.Data)
			}
			if n.Type == html.ElementNode && n.Data == "br" {
				b.WriteByte('\n')
			}
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				walk(child)
			}
		}
		walk(node)
		return b.String()
	}
	pres := []string{}
	var findPre func(*html.Node)
	findPre = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "pre" {
			pres = append(pres, text(n))
			return
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			findPre(child)
		}
	}
	findPre(root)
	if len(pres) > 0 {
		return strings.Join(pres, "\n"), nil
	}
	var output strings.Builder
	emit := func(prefix, value string) {
		for _, line := range strings.Split(strings.TrimSuffix(value, "\n"), "\n") {
			output.WriteString(prefix)
			output.WriteString(line)
			output.WriteByte('\n')
		}
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "tr" {
			contextWritten := false
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				if child.Type != html.ElementNode || child.Data != "td" {
					continue
				}
				classes := ""
				for _, attribute := range child.Attr {
					if attribute.Key == "class" {
						classes = " " + attribute.Val + " "
					}
				}
				switch {
				case strings.Contains(classes, " diff-deletedline "):
					emit("-", text(child))
				case strings.Contains(classes, " diff-addedline "):
					emit("+", text(child))
				case strings.Contains(classes, " diff-context ") && !contextWritten:
					emit(" ", text(child))
					contextWritten = true
				}
			}
			return
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	if output.Len() == 0 {
		return text(root), nil
	}
	return output.String(), nil
}
