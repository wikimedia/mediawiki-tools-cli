package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mwclient "cgt.name/pkg/go-mwclient"
)

func TestAuditFindsRetiredPagesWithoutTreatingCommunityPagesAsGenerated(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(filepath.Join(outputDir, "pages.json"), []document{{Title: "Cli/guide/Kept_page"}}); err != nil {
		t.Fatal(err)
	}
	published := snapshot{Revision: 41, Text: marked("Original"), User: "Publisher", Comment: publicationSummary}
	pages := map[string]snapshot{
		"Cli/guide/Old":                    published,
		"Cli/fr":                           {Revision: 46, Text: published.Text, Comment: publicationSummary, Translation: true},
		"Cli/ref/mw docker docker-compose": published,
		"Cli/ref/mw gitlab ci artifact":    published,
		"Cli/ref/mw toolhub":               published,
		"Cli/guide/Edited": {
			Revision: 42, Text: strings.Replace(published.Text, "Original", "Corrected", 1), User: "Human", Comment: "Fix wording",
		},
		"Cli/guide/Community": {Revision: 43, Text: "Community guide", User: "Human", Comment: "Write guide"},
		"Cli/ref/Legacy":      {Revision: 44, Text: "Legacy reference", User: "Addbot", Comment: "Legacy publish"},
		"Cli/ref/Redirect":    {Revision: 45, Text: marked("#REDIRECT [[Cli/ref/New]]"), User: "Publisher", Comment: publicationSummary},
	}
	lists := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Query().Get("action") != "query" {
			t.Error("audit attempted a write or login")
			http.Error(w, "read-only audit", http.StatusForbidden)
			return
		}
		query := r.URL.Query()
		if query.Get("list") == "allpages" {
			lists++
			if query.Get("apprefix") != "Cli/" || query.Get("apnamespace") != "0" {
				t.Error("incorrect audit scope")
			}
			titles := []string{"Cli/guide/Kept page", "Cli/guide/Old", "Cli/guide/Edited"}
			response := map[string]interface{}{}
			if query.Get("apcontinue") == "" {
				response["continue"] = map[string]string{"apcontinue": "next", "continue": "-||"}
			} else {
				titles = []string{"Cli/guide/Community", "Cli/ref/Legacy", "Cli/ref/Redirect", "Cli/fr", "Cli/ref/mw docker docker-compose", "Cli/ref/mw toolhub", "Cli/ref/mw gitlab ci artifact"}
			}
			records := []interface{}{}
			for _, title := range titles {
				records = append(records, map[string]interface{}{"title": title})
			}
			response["query"] = map[string]interface{}{"allpages": records}
			_ = json.NewEncoder(w).Encode(response)
			return
		}
		if query.Get("revids") == "41" {
			replyRevisions(w, "Cli/guide/Edited", []snapshot{published}, "")
			return
		}
		title := query.Get("titles")
		if query.Get("rvslots") == "" {
			revisions := []snapshot{pages[title]}
			if title == "Cli/guide/Edited" {
				revisions = append(revisions, published)
			}
			replyRevisions(w, title, revisions, "")
			return
		}
		records := []interface{}{}
		for _, title := range strings.Split(title, "|") {
			page, ok := pages[title]
			if !ok {
				t.Errorf("audit fetched a managed or unknown page: %s", title)
			}
			properties := map[string]string{}
			if page.Translation {
				properties["translate-is-translation"] = ""
			}
			records = append(records, map[string]interface{}{"title": title, "pageprops": properties, "revisions": []interface{}{map[string]interface{}{"revid": page.Revision, "user": page.User, "comment": page.Comment, "slots": map[string]interface{}{"main": map[string]interface{}{"content": page.Text}}}}})
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"curtimestamp": "2026-10-18T00:00:00Z", "query": map[string]interface{}{"pages": records}})
	}))
	defer server.Close()
	config := configuration{API: server.URL, PageURL: server.URL + "/wiki", ReferenceRoot: "Cli/ref", LegacyPublisher: "Addbot", LegacySummary: "Legacy publish"}
	if err := auditDocumentation(config); err != nil {
		t.Fatal(err)
	}
	report, err := os.ReadFile(filepath.Join(outputDir, "wiki", "audit.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"| OBSOLETE | [Cli/guide/Old]", "| OBSOLETE-EDITED | [Cli/guide/Edited]", "| UNMANAGED | [Cli/guide/Community]", "| OBSOLETE | [Cli/ref/Legacy]", "| OBSOLETE-REDIRECT | [Cli/ref/Redirect]", "| TRANSLATION | [Cli/fr]", "| ALIAS | [Cli/ref/mw docker docker-compose]", "| HIDDEN-COMMAND | [Cli/ref/mw toolhub]", "| DEPRECATED-COMMAND | [Cli/ref/mw gitlab ci artifact]"} {
		if !strings.Contains(string(report), expected) {
			t.Fatalf("report missing %q:\n%s", expected, report)
		}
	}
	retired, _, _ := strings.Cut(string(report), "## Retained wiki content")
	if strings.Contains(retired, "[Cli/fr]") || strings.Contains(retired, "[Cli/ref/mw toolhub]") || strings.Contains(retired, "[Cli/ref/mw docker docker-compose]") {
		t.Fatal("retained wiki content was offered for retirement")
	}
	if lists != 2 || strings.Contains(string(report), "Kept page") {
		t.Fatal("audit did not paginate or normalize managed titles")
	}
}

func TestAuditRejectsRepeatedPageContinuation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"continue":{"apcontinue":"same"},"query":{"allpages":[]}}`))
	}))
	defer server.Close()
	client, err := mwclient.New(server.URL, "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := listDocumentationPages(client, "Cli/"); err == nil || !strings.Contains(err.Error(), "repeated") {
		t.Fatalf("invalid pagination was not rejected: %v", err)
	}
}
