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

type wikiPage struct {
	Title     string `json:"title"`
	Missing   bool   `json:"missing"`
	Revisions []struct {
		ID      int64  `json:"revid"`
		User    string `json:"user"`
		Comment string `json:"comment"`
		Slots   map[string]struct {
			Content *string `json:"content"`
			Hidden  bool    `json:"texthidden"`
		} `json:"slots"`
	} `json:"revisions"`
}

type snapshot struct {
	Revision int64  `json:"revision"`
	Text     string `json:"text"`
	User     string `json:"user"`
	Comment  string `json:"comment"`
	Missing  bool   `json:"missing"`
}

type review struct {
	API       string              `json:"api"`
	Timestamp string              `json:"timestamp"`
	Pages     map[string]snapshot `json:"pages"`
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
		response, err := client.Get(params.Values{"action": "query", "formatversion": "2", "curtimestamp": "1", "prop": "revisions", "rvprop": "ids|content|user|comment", "rvslots": "main", "titles": strings.Join(titles[start:end], "|")})
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
			item := snapshot{Missing: page.Missing}
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

func safeToPublish(config configuration, title string, current snapshot, desired string) bool {
	desiredBody, _ := unmark(desired)
	if current.Missing || normalize(current.Text) == normalize(desired) || normalize(current.Text) == normalize(desiredBody) {
		return true
	}
	if _, valid := unmark(current.Text); valid {
		return true
	}
	if baseline, ok := config.Reviewed[title]; ok && baseline.SHA256 == digest(normalize(current.Text)) {
		return true
	}
	// Bootstrap existing generated pages only if their current revision is the
	// known publishing bot's generated edit. Community revisions need review.
	return !strings.Contains(current.Text, markerPrefix) &&
		strings.HasPrefix(title, config.ReferenceRoot+"/") && config.LegacyPublisher != "" && config.LegacySummary != "" &&
		current.User == config.LegacyPublisher && current.Comment == config.LegacySummary
}

// A human revert can restore exactly the last bot publication. Compare the
// content, rather than trusting a revert summary or treating every revert as drift.
func unchangedLegacyReference(client *mwclient.Client, config configuration, title string, current snapshot) (bool, error) {
	if config.LegacyPublisher == "" || config.LegacySummary == "" || !strings.HasPrefix(title, config.ReferenceRoot+"/") || strings.Contains(current.Text, markerPrefix) {
		return false, nil
	}
	response, err := client.Get(params.Values{"action": "query", "formatversion": "2", "prop": "revisions", "rvprop": "content|user|comment", "rvslots": "main", "rvuser": config.LegacyPublisher, "rvlimit": "1", "titles": title})
	if err != nil {
		return false, err
	}
	data, err := response.Marshal()
	if err != nil {
		return false, err
	}
	var reply struct {
		Query struct {
			Pages []wikiPage `json:"pages"`
		} `json:"query"`
	}
	if err := json.Unmarshal(data, &reply); err != nil {
		return false, err
	}
	if len(reply.Query.Pages) != 1 || len(reply.Query.Pages[0].Revisions) != 1 {
		return false, nil
	}
	revision := reply.Query.Pages[0].Revisions[0]
	slot, readable := revision.Slots["main"]
	return readable && slot.Content != nil && !slot.Hidden && revision.User == config.LegacyPublisher && revision.Comment == config.LegacySummary && normalize(*slot.Content) == normalize(current.Text), nil
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
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(directory, "review.json"), state); err != nil {
		return err
	}
	conflicts := 0
	changed := 0
	var report strings.Builder
	report.WriteString("# Wiki documentation review\n\n| Status | Page | Diff |\n| --- | --- | --- |\n")
	for i, document := range documents {
		desired, err := os.ReadFile(document.Wikitext)
		if err != nil {
			return err
		}
		current := state.Pages[document.Title]
		if normalize(current.Text) == normalize(string(desired)) && !current.Missing {
			continue
		}
		changed++
		status := "UPDATE"
		if current.Missing {
			status = "CREATE"
		} else if !safeToPublish(config, document.Title, current, string(desired)) {
			unchanged, err := unchangedLegacyReference(client, config, document.Title, current)
			if err != nil {
				return err
			}
			if !unchanged {
				status = "WIKI-EDIT"
				conflicts++
			}
		}
		stem := filepath.Join(directory, fmt.Sprintf("%03d", i))
		if err := saveReview(stem, current.Text, document.Wikitext); err != nil {
			return err
		}
		if mode == "pull" && !current.Missing && document.Title != config.ReferenceRoot && !strings.HasPrefix(document.Title, config.ReferenceRoot+"/") {
			body, _ := unmark(current.Text)
			markdown, err := convertWiki([]byte(body), strings.TrimPrefix(document.Source, "docs/"), config)
			if err != nil {
				return err
			}
			if err := os.WriteFile(stem+".md", markdown, 0o644); err != nil {
				return err
			}
		}
		fmt.Printf("%s %s (%s.diff)\n", status, document.Title, stem)
		fmt.Fprintf(&report, "| %s | [%s](%s) | [%s.diff](%s.diff) |\n", status, document.Title, wikiURL(config, document.Title), filepath.Base(stem), filepath.Base(stem))
	}
	if err := os.WriteFile(filepath.Join(directory, "report.md"), []byte(report.String()), 0o644); err != nil {
		return err
	}
	fmt.Printf("%d pages differ; %d have unreviewed wiki edits. Review files: %s\n", changed, conflicts, directory)
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
	summary := "Publish a Wikimedia CLI documentation"
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
	return os.WriteFile(stem+".diff", diff, 0o644)
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
