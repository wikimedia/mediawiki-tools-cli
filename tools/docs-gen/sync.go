package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strconv"
	"strings"

	mwclient "cgt.name/pkg/go-mwclient"
	"cgt.name/pkg/go-mwclient/params"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

type wikiRevision struct {
	ID      int64  `json:"revid"`
	User    string `json:"user"`
	Comment string `json:"comment"`
	Slots   map[string]struct {
		Content *string `json:"content"`
		Hidden  bool    `json:"texthidden"`
	} `json:"slots"`
}

type wikiPage struct {
	Title     string            `json:"title"`
	Missing   bool              `json:"missing"`
	Revisions []wikiRevision    `json:"revisions"`
	PageProps map[string]string `json:"pageprops"`
}

type snapshot struct {
	Revision    int64  `json:"revision"`
	Text        string `json:"text"`
	User        string `json:"user"`
	Comment     string `json:"comment"`
	Missing     bool   `json:"missing"`
	Translation bool   `json:"translation,omitempty"`
}

type review struct {
	API       string              `json:"api"`
	Timestamp string              `json:"timestamp"`
	Pages     map[string]snapshot `json:"pages"`
	Baselines map[string]snapshot `json:"baselines,omitempty"`
}

func newWiki(config configuration) (*mwclient.Client, error) {
	return mwclient.New(config.API, cli.HTTPUserAgent)
}

func fetchPages(client *mwclient.Client, titles []string) (review, error) {
	result := review{Pages: map[string]snapshot{}}
	for start := 0; start < len(titles); start += 40 {
		end := start + 40
		if end > len(titles) {
			end = len(titles)
		}
		response, err := client.Get(params.Values{"action": "query", "formatversion": "2", "curtimestamp": "1", "prop": "revisions|pageprops", "ppprop": "translate-is-translation", "rvprop": "ids|content|user|comment", "rvslots": "main", "titles": strings.Join(titles[start:end], "|")})
		if err != nil {
			return result, err
		}
		data, err := response.Marshal()
		if err != nil {
			return result, err
		}
		var reply struct {
			Timestamp string `json:"curtimestamp"`
			Query     struct {
				Pages []wikiPage `json:"pages"`
			} `json:"query"`
		}
		if err := json.Unmarshal(data, &reply); err != nil {
			return result, err
		}
		if reply.Timestamp == "" {
			return result, fmt.Errorf("wiki response has no edit start timestamp")
		}
		if result.Timestamp == "" {
			result.Timestamp = reply.Timestamp
		}
		found := map[string]snapshot{}
		for _, page := range reply.Query.Pages {
			_, translation := page.PageProps["translate-is-translation"]
			item := snapshot{Missing: page.Missing, Translation: translation}
			if !page.Missing {
				if len(page.Revisions) != 1 {
					return result, fmt.Errorf("cannot read revision for %s", page.Title)
				}
				revision := page.Revisions[0]
				slot, ok := revision.Slots["main"]
				if !ok || slot.Hidden || slot.Content == nil || revision.ID <= 0 {
					return result, fmt.Errorf("cannot read content of %s", page.Title)
				}
				item.Revision, item.Text, item.User, item.Comment = revision.ID, *slot.Content, revision.User, revision.Comment
			}
			found[strings.ReplaceAll(page.Title, "_", " ")] = item
		}
		for _, title := range titles[start:end] {
			page, ok := found[strings.ReplaceAll(title, "_", " ")]
			if !ok {
				return result, fmt.Errorf("wiki response omitted %s", title)
			}
			result.Pages[title] = page
		}
	}
	return result, nil
}

func safeToPublish(config configuration, title string, current snapshot, desired string, published *snapshot) bool {
	if current.Missing || documentationBody(current.Text) == documentationBody(desired) {
		return true
	}
	if reviewed, ok := config.Reviewed[title]; ok && reviewed.SHA256 == digest(normalize(current.Text)) {
		return true
	}
	return published != nil && documentationBody(published.Text) == documentationBody(current.Text)
}

func editParameters(title, text, summary, timestamp string, current snapshot) params.Values {
	values := params.Values{"title": title, "text": text, "summary": summary, "bot": "1", "starttimestamp": timestamp}
	if current.Missing {
		values["createonly"] = "1"
	} else {
		values["baserevid"] = strconv.FormatInt(current.Revision, 10)
		values["nocreate"] = "1"
	}
	return values
}

func synchronize(config configuration, mode string) error {
	var documents []document
	if err := readJSON(filepath.Join(outputDir, "pages.json"), &documents); err != nil {
		return fmt.Errorf("run make docs first: %w", err)
	}
	client, err := newWiki(config)
	if err != nil {
		return err
	}
	titles := make([]string, len(documents))
	for i, document := range documents {
		titles[i] = document.Title
	}
	state, err := fetchPages(client, titles)
	if err != nil {
		return err
	}
	state.API = config.API
	directory := filepath.Join(outputDir, "wiki")
	if err := os.RemoveAll(directory); err != nil {
		return err
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	state.Baselines = map[string]snapshot{}
	conflicts, changed, wikiChanges, laterRevisions := 0, 0, 0, 0
	var report, preview strings.Builder
	report.WriteString("# Edits since the last automatic publication\n\n| Status | Page | Revisions | Wiki edits |\n| --- | --- | --- | --- |\n")
	preview.WriteString("# Proposed publication changes\n\n| Status | Page | Diff |\n| --- | --- | --- |\n")
	for i, document := range documents {
		desired, err := os.ReadFile(document.Wikitext)
		if err != nil {
			return err
		}
		current := state.Pages[document.Title]
		published, err := lastPublication(client, config, document.Title, current)
		if err != nil {
			return fmt.Errorf("%s: %w", document.Title, err)
		}
		if published != nil {
			state.Baselines[document.Title] = *published
			if current.Revision > published.Revision {
				laterRevisions++
			}
		}
		safe := safeToPublish(config, document.Title, current, string(desired), published)
		if !safe {
			conflicts++
		}
		stem := filepath.Join(directory, fmt.Sprintf("%03d", i))
		differs := current.Missing || normalize(current.Text) != normalize(string(desired))
		if differs {
			changed++
			status := "UPDATE"
			if current.Missing {
				status = "CREATE"
			}
			if err := saveReview(stem, current.Text, document.Wikitext); err != nil {
				return err
			}
			fmt.Fprintf(&preview, "| %s | [%s](%s) | [%s.publish.diff](%s.publish.diff) |\n", status, document.Title, wikiURL(config, document.Title), filepath.Base(stem), filepath.Base(stem))
		}
		if published != nil && documentationBody(published.Text) != documentationBody(current.Text) {
			wikiChanges++
			if err := saveWikiEdits(stem, *published, current); err != nil {
				return err
			}
			status := "WIKI-EDIT"
			if safe {
				status = "REVIEWED"
			}
			fmt.Printf("%s %s (%s.wiki.diff; revisions %d → %d)\n", status, document.Title, stem, published.Revision, current.Revision)
			comparisonURL := fmt.Sprintf("%s?oldid=%d&diff=%d", wikiURL(config, document.Title), published.Revision, current.Revision)
			fmt.Fprintf(&report, "| %s | [%s](%s) | [%d → %d](%s) | [%s.wiki.diff](%s.wiki.diff) |\n", status, document.Title, wikiURL(config, document.Title), published.Revision, current.Revision, comparisonURL, filepath.Base(stem), filepath.Base(stem))
		} else if !safe {
			fmt.Printf("NO-BASELINE %s (review %s.wiki)\n", document.Title, stem)
			fmt.Fprintf(&report, "| NO-BASELINE | [%s](%s) | %d | [%s.wiki](%s.wiki) |\n", document.Title, wikiURL(config, document.Title), current.Revision, filepath.Base(stem), filepath.Base(stem))
		}
		if mode == "pull" && differs && !current.Missing && document.Title != config.ReferenceRoot && !strings.HasPrefix(document.Title, config.ReferenceRoot+"/") {
			body, _ := unmark(current.Text)
			markdown, err := convertWiki([]byte(body), strings.TrimPrefix(document.Source, "docs/"), config)
			if err != nil {
				return err
			}
			if err := os.WriteFile(stem+".md", markdown, 0o644); err != nil {
				return err
			}
		}
	}
	if err := writeJSON(filepath.Join(directory, "review.json"), state); err != nil {
		return err
	}
	if wikiChanges == 0 && conflicts == 0 {
		report.WriteString("\nNo remaining content changes since automatic publication or the initial reviewed import.\n")
	}
	report.WriteString("\n[Full publication preview](publish-report.md) includes changes from the new release.\n")
	for name, body := range map[string]string{"report.md": report.String(), "publish-report.md": preview.String()} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(body), 0o644); err != nil {
			return err
		}
	}
	fmt.Printf("Checked %d pages: %d have later wiki revisions, %d have content changes, %d need review.\n", len(documents), laterRevisions, wikiChanges, conflicts)
	fmt.Printf("Wiki edits: %s/report.md; %d proposed publication changes: %s/publish-report.md\n", directory, changed, directory)
	if mode == "pull" {
		return nil
	}
	if conflicts > 0 {
		return fmt.Errorf("publishing blocked: review wiki edits, import guide corrections or fix command sources, then run make docs-accept page=TITLE")
	}
	if mode == "check" || changed == 0 {
		return nil
	}
	username, password := os.Getenv("DOCS_UPDATE_USER"), os.Getenv("DOCS_UPDATE_PASS")
	if username == "" || password == "" {
		return fmt.Errorf("set DOCS_UPDATE_USER and DOCS_UPDATE_PASS (MediaWiki bot password) to publish")
	}
	if err := client.Login(username, password); err != nil {
		return err
	}
	client.Assert = mwclient.AssertUser
	summary := publicationSummary
	if version := os.Getenv("DOCS_VERSION"); version != "" {
		summary += " (" + version + ")"
	}
	for _, document := range documents {
		desired, err := os.ReadFile(document.Wikitext)
		if err != nil {
			return err
		}
		current := state.Pages[document.Title]
		if !current.Missing && normalize(current.Text) == normalize(string(desired)) {
			continue
		}
		if err := client.Edit(editParameters(document.Title, string(desired), summary, state.Timestamp, current)); err != nil && !errors.Is(err, mwclient.ErrEditNoChange) {
			return fmt.Errorf("publishing %s: %w", document.Title, err)
		}
		fmt.Println("Published", document.Title)
	}
	return nil
}

func saveReview(stem, current, proposed string) error {
	if err := os.WriteFile(stem+".wiki", []byte(current), 0o644); err != nil {
		return err
	}
	command := osexec.Command("git", "--no-pager", "diff", "--no-index", "--", stem+".wiki", proposed)
	diff, err := command.CombinedOutput()
	if err != nil {
		var exit *osexec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 {
			return fmt.Errorf("generate wiki diff: %w: %s", err, diff)
		}
	}
	return os.WriteFile(stem+".publish.diff", diff, 0o644)
}

func saveWikiEdits(stem string, published, current snapshot) error {
	before, after := stem+".published.wiki", stem+".edited.wiki"
	if err := os.WriteFile(before, []byte(documentationBody(published.Text)), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(after, []byte(documentationBody(current.Text)), 0o644); err != nil {
		return err
	}
	command := osexec.Command("git", "--no-pager", "diff", "--no-index", "--", before, after)
	diff, err := command.CombinedOutput()
	if err != nil {
		var exit *osexec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 {
			return fmt.Errorf("generate community edit diff: %w: %s", err, diff)
		}
	}
	return os.WriteFile(stem+".wiki.diff", diff, 0o644)
}

func acceptReview(config configuration, title string) error {
	var state review
	if err := readJSON(filepath.Join(outputDir, "wiki", "review.json"), &state); err != nil {
		return err
	}
	if config.API != state.API {
		return fmt.Errorf("review belongs to a different wiki")
	}
	page, ok := state.Pages[title]
	if title == "" || !ok || page.Missing {
		return fmt.Errorf("no existing reviewed wiki page %q; run make docs-check first", title)
	}
	if config.Reviewed == nil {
		config.Reviewed = map[string]baseline{}
	}
	config.Reviewed[title] = baseline{page.Revision, digest(normalize(page.Text))}
	return writeJSON(configPath, config)
}

func importGuide(config configuration, title, source string) error {
	target, err := guidePath(source)
	if err != nil {
		return err
	}
	if title == "" || title == config.ReferenceRoot || strings.HasPrefix(title, config.ReferenceRoot+"/") {
		return fmt.Errorf("import a guide title, not generated reference")
	}
	for _, existing := range config.Guides {
		if (existing.Title == title) != (existing.Source == source) {
			return fmt.Errorf("title or source already belongs to a different guide")
		}
	}
	client, err := newWiki(config)
	if err != nil {
		return err
	}
	state, err := fetchPages(client, []string{title})
	if err != nil {
		return err
	}
	page := state.Pages[title]
	if page.Missing {
		return fmt.Errorf("wiki page %s does not exist", title)
	}
	body, _ := unmark(page.Text)
	markdown, err := convertWiki([]byte(body), source, config)
	if err != nil {
		return err
	}
	attribution := fmt.Sprintf("<!-- Imported from %s?oldid=%d (CC BY-SA 4.0); see the page history for contributors. -->\n\n", wikiURL(config, title), page.Revision)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	// Preserve a local draft before importing over an existing guide.
	if previous, err := os.ReadFile(target); err == nil {
		if err := os.WriteFile(target+".bak", previous, 0o644); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.WriteFile(target, append([]byte(attribution), markdown...), 0o644); err != nil {
		return err
	}
	found := false
	for _, guide := range config.Guides {
		found = found || guide.Title == title
	}
	if !found {
		config.Guides = append(config.Guides, guide{source, title})
	}
	if config.Reviewed == nil {
		config.Reviewed = map[string]baseline{}
	}
	config.Reviewed[title] = baseline{page.Revision, digest(normalize(page.Text))}
	if err := writeJSON(configPath, config); err != nil {
		return err
	}
	fmt.Printf("Imported %s revision %d into %s; review the Markdown conversion before committing.\n", title, page.Revision, target)
	return nil
}
