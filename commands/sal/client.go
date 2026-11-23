package sal

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"golang.org/x/net/html"
)

// Entry is one public SAL record. Atom does not identify individual projects in
// the all-project feed, so Project is omitted there rather than guessed.
type Entry struct {
	Timestamp string `json:"timestamp"`
	User      string `json:"user"`
	Project   string `json:"project,omitempty"`
	Message   string `json:"message"`
	URL       string `json:"url"`
}

func readEntries(ctx context.Context, u *url.URL, project string, search bool) ([]Entry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	if search {
		req.Header.Set("Accept", "text/html")
	} else {
		req.Header.Set("Accept", "application/atom+xml")
	}
	client := cli.NewHTTPClient()
	client.Timeout = 30 * time.Second
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("read SAL: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("read SAL: HTTP %d", resp.StatusCode)
	}
	const maxBody = 8 << 20
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxBody {
		return nil, fmt.Errorf("SAL response exceeds 8 MiB")
	}
	if search {
		return parseSearch(string(body))
	}
	return parseAtom(body, project)
}

func parseAtom(body []byte, project string) ([]Entry, error) {
	var feed struct {
		XMLName xml.Name `xml:"http://www.w3.org/2005/Atom feed"`
		Entries []struct {
			Title     string `xml:"title"`
			Published string `xml:"published"`
			Author    string `xml:"author>name"`
			ID        string `xml:"id"`
		} `xml:"entry"`
	}
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("decode SAL Atom feed: %w", err)
	}
	entries := make([]Entry, 0, len(feed.Entries))
	if project == "__all__" {
		project = ""
	}
	for _, item := range feed.Entries {
		entries = append(entries, Entry{Timestamp: item.Published, User: item.Author, Project: project, Message: item.Title, URL: item.ID})
	}
	return entries, nil
}

// The public Atom route ignores search parameters. For archive searches use
// the same semantic table cells as the SAL web UI, retaining permalink IDs.
// Source: https://github.com/bd808/SAL/blob/master/src/Pages/Sal.php
func parseSearch(body string) ([]Entry, error) {
	root, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("decode SAL search: %w", err)
	}
	entries := []Entry{}
	date := ""
	foundTable := false
	var walk func(*html.Node) error
	walk = func(n *html.Node) error {
		if n.Type == html.ElementNode && n.Data == "table" && hasClass(n, "sal") {
			foundTable = true
		}
		if n.Type == html.ElementNode && n.Data == "a" && hasClass(n, "day") {
			date = strings.TrimSpace(nodeText(n))
		}
		if n.Type == html.ElementNode && n.Data == "tr" {
			entry := Entry{}
			clock := ""
			cells := 0
			for cell := n.FirstChild; cell != nil; cell = cell.NextSibling {
				if cell.Type != html.ElementNode || cell.Data != "td" {
					continue
				}
				cells++
				switch {
				case hasClass(cell, "time"):
					clock = strings.TrimSpace(nodeText(cell))
					for a := cell.FirstChild; a != nil; a = a.NextSibling {
						if a.Type == html.ElementNode && a.Data == "a" {
							for _, attr := range a.Attr {
								if attr.Key == "href" && strings.HasPrefix(attr.Val, "/log/") {
									entry.URL = baseURL + attr.Val
								}
							}
						}
					}
				case hasClass(cell, "nick"):
					entry.User = strings.Trim(strings.TrimSpace(nodeText(cell)), "<>")
				case hasClass(cell, "project"):
					entry.Project = strings.Trim(strings.TrimSpace(nodeText(cell)), "[]")
				case hasClass(cell, "message"):
					entry.Message = strings.TrimSpace(nodeText(cell))
				}
			}
			if cells > 0 {
				stamp, err := time.Parse("2006-01-02 15:04", date+" "+clock)
				if err != nil || entry.URL == "" || entry.User == "" || entry.Project == "" {
					return fmt.Errorf("SAL search response has an unrecognised log row")
				}
				entry.Timestamp = stamp.Format(time.RFC3339)
				entries = append(entries, entry)
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			if err := walk(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(root); err != nil {
		return nil, err
	}
	if !foundTable {
		return nil, fmt.Errorf("SAL search response has no log table; the service's page format may have changed")
	}
	return entries, nil
}

func hasClass(n *html.Node, class string) bool {
	for _, attr := range n.Attr {
		if attr.Key == "class" {
			for _, value := range strings.Fields(attr.Val) {
				if value == class {
					return true
				}
			}
		}
	}
	return false
}

func nodeText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var text strings.Builder
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		text.WriteString(nodeText(child))
	}
	return text.String()
}
