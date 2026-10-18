package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	mwclient "cgt.name/pkg/go-mwclient"
	"cgt.name/pkg/go-mwclient/params"
)

// List redirects as well as ordinary pages; an old URL may intentionally remain useful.
func listDocumentationPages(client *mwclient.Client, prefix string) ([]string, error) {
	values := params.Values{"action": "query", "formatversion": "2", "list": "allpages", "apprefix": prefix, "apnamespace": "0", "aplimit": "500"}
	var titles []string
	seen := map[string]bool{}
	for {
		response, err := client.Get(values)
		if err != nil {
			return nil, err
		}
		data, err := response.Marshal()
		if err != nil {
			return nil, err
		}
		var reply struct {
			Continue map[string]string `json:"continue"`
			Query    struct {
				Pages []struct {
					Title string `json:"title"`
				} `json:"allpages"`
			} `json:"query"`
		}
		if err := json.Unmarshal(data, &reply); err != nil {
			return nil, err
		}
		if reply.Query.Pages == nil {
			return nil, fmt.Errorf("wiki response omitted the page list for %s", prefix)
		}
		for _, page := range reply.Query.Pages {
			if !strings.HasPrefix(page.Title, prefix) {
				return nil, fmt.Errorf("wiki returned a page outside %s: %s", prefix, page.Title)
			}
			titles = append(titles, page.Title)
		}
		next := reply.Continue["apcontinue"]
		if next == "" {
			return titles, nil
		}
		if seen[next] {
			return nil, fmt.Errorf("repeated page-list continuation for %s", prefix)
		}
		seen[next] = true
		for key, value := range reply.Continue {
			values[key] = value
		}
	}
}

func auditDocumentation(config configuration) error {
	var documents []document
	if err := readJSON(filepath.Join(outputDir, "pages.json"), &documents); err != nil {
		return fmt.Errorf("run make docs first: %w", err)
	}
	client, err := newWiki(config)
	if err != nil {
		return err
	}
	// Guide and reference URLs share the project's top-level wiki subtree.
	prefix := strings.Split(config.ReferenceRoot, "/")[0] + "/"
	titles, err := listDocumentationPages(client, prefix)
	if err != nil {
		return err
	}
	managed := map[string]bool{}
	for _, item := range documents {
		managed[strings.ReplaceAll(item.Title, "_", " ")] = true
	}
	var candidates []string
	for _, title := range titles {
		if !managed[strings.ReplaceAll(title, "_", " ")] {
			candidates = append(candidates, title)
		}
	}
	state, err := fetchPages(client, candidates)
	if err != nil {
		return err
	}
	var report strings.Builder
	report.WriteString("# Wiki pages outside the current documentation build\n\nPreviously published pages may need a redirect or retirement. UNMANAGED pages have no detected automatic publication and may be community documentation. Redirects may deliberately preserve old links. Nothing is moved or deleted automatically.\n\n| Status | Page | Latest revision |\n| --- | --- | --- |\n")
	obsolete := 0
	for _, title := range candidates {
		current := state.Pages[title]
		if current.Missing {
			continue // A page may be deleted between listing and reading.
		}
		published, err := lastPublication(client, config, title, current)
		if err != nil {
			return fmt.Errorf("%s: %w", title, err)
		}
		status := "UNMANAGED"
		if strings.Contains(current.Text, markerPrefix) || (published != nil && automaticPublication(config, title, *published)) {
			status = "OBSOLETE"
			obsolete++
			_, valid := unmark(current.Text)
			if (published != nil && documentationBody(published.Text) != documentationBody(current.Text)) || (strings.Contains(current.Text, markerPrefix) && !valid) {
				status = "OBSOLETE-EDITED"
			}
		}
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(current.Text)), "#REDIRECT") {
			status += "-REDIRECT"
		}
		fmt.Fprintf(&report, "| %s | [%s](%s) | [%d](%s?oldid=%d) |\n", status, title, wikiURL(config, title), current.Revision, wikiURL(config, title), current.Revision)
	}
	if len(candidates) == 0 {
		report.WriteString("\nAll existing documentation pages are in the current build.\n")
	}
	file := filepath.Join(outputDir, "wiki", "audit.md")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(file, []byte(report.String()), 0o644); err != nil {
		return err
	}
	fmt.Printf("Audited %d existing wiki pages: %d outside the build, %d previously published; review %s. No pages changed.\n", len(titles), len(candidates), obsolete, file)
	return nil
}
