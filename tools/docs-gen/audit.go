package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	mwclient "cgt.name/pkg/go-mwclient"
	"cgt.name/pkg/go-mwclient/params"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/cmd"
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
	commands := referenceCommands(cmd.NewMwCliCmd())
	var retired, retained strings.Builder
	obsolete := 0
	for _, title := range candidates {
		current := state.Pages[title]
		if current.Missing {
			continue // A page may be deleted between listing and reading.
		}
		status := "UNMANAGED"
		reference := commands[strings.TrimPrefix(title, config.ReferenceRoot+"/")]
		if current.Translation {
			status = "TRANSLATION"
		} else if strings.HasPrefix(title, config.ReferenceRoot+"/") && reference.Canonical != "" {
			status = "SUPPORTED-COMMAND"
			if strings.TrimPrefix(title, config.ReferenceRoot+"/") != strings.ReplaceAll(reference.Canonical, "_", " ") {
				status = "ALIAS"
			}
			if reference.Deprecated {
				status = "DEPRECATED-COMMAND"
			}
			if reference.Hidden {
				status = "HIDDEN-COMMAND"
			}
		} else {
			published, err := lastPublication(client, config, title, current)
			if err != nil {
				return fmt.Errorf("%s: %w", title, err)
			}
			if strings.Contains(current.Text, markerPrefix) || (published != nil && automaticPublication(config, title, *published)) {
				status = "OBSOLETE"
				obsolete++
				_, valid := unmark(current.Text)
				if (published != nil && documentationBody(published.Text) != documentationBody(current.Text)) || (strings.Contains(current.Text, markerPrefix) && !valid) {
					status = "OBSOLETE-EDITED"
				}
			}
		}
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(current.Text)), "#REDIRECT") {
			status += "-REDIRECT"
		}
		section := &retained
		if strings.HasPrefix(status, "OBSOLETE") {
			section = &retired
		}
		fmt.Fprintf(section, "| %s | [%s](%s) | [%d](%s?oldid=%d) |\n", status, title, wikiURL(config, title), current.Revision, wikiURL(config, title), current.Revision)
	}
	var report strings.Builder
	fmt.Fprintf(&report, "# Wiki page audit\n\n%d existing pages are outside the build; %d are previously generated pages needing review. Nothing is moved or deleted automatically.\n\n## Retirement candidates\n\nReview these for redirects or retirement. OBSOLETE-EDITED means content changed after publication; preserve those corrections. Redirects may intentionally preserve useful links.\n\n| Status | Page | Latest revision |\n| --- | --- | --- |\n", len(candidates), obsolete)
	report.WriteString(retired.String())
	if obsolete == 0 {
		report.WriteString("\nNo retirement candidates found.\n")
	}
	report.WriteString("\n## Retained wiki content\n\nTRANSLATION pages are maintained through Translate. HIDDEN-COMMAND and DEPRECATED-COMMAND references describe commands still present but excluded from generated help. SUPPORTED-COMMAND references are other existing commands; ALIAS references describe another name for one. UNMANAGED pages have no verified automatic publication, and may include community guides, metadata and navigation redirects. None is a retirement candidate.\n\n| Status | Page | Latest revision |\n| --- | --- | --- |\n")
	report.WriteString(retained.String())
	file := filepath.Join(outputDir, "wiki", "audit.md")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(file, []byte(report.String()), 0o644); err != nil {
		return err
	}
	fmt.Printf("Audited %d existing wiki pages: %d outside the build, %d retirement candidates; review %s. No pages changed.\n", len(titles), len(candidates), obsolete, file)
	return nil
}

// Canonical names, aliases and hidden commands remain valid even when Cobra
// omits them from the generated reference. Normalize underscores as MediaWiki does.
type referenceCommand struct {
	Canonical  string
	Hidden     bool
	Deprecated bool
}

func referenceCommands(root *cobra.Command) map[string]referenceCommand {
	result := map[string]referenceCommand{}
	var visit func(*cobra.Command, []string, bool)
	visit = func(command *cobra.Command, parents []string, hidden bool) {
		hidden = hidden || command.Hidden
		names := append([]string{command.Name()}, command.Aliases...)
		var prefixes []string
		for _, parent := range parents {
			for _, name := range names {
				full := strings.TrimSpace(parent + " " + name)
				prefixes = append(prefixes, full)
				result[strings.ReplaceAll(full, "_", " ")] = referenceCommand{command.CommandPath(), hidden, command.Deprecated != ""}
			}
		}
		for _, child := range command.Commands() {
			visit(child, prefixes, hidden)
		}
	}
	visit(root, []string{""}, false)
	return result
}
