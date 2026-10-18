package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"

	mwclient "cgt.name/pkg/go-mwclient"
)

func TestWikiEditDetection(t *testing.T) {
	config := configuration{Reviewed: map[string]baseline{"Guide": {42, digest(normalize("reviewed"))}}}
	for _, tc := range []struct {
		name, title, current string
		published            *snapshot
		safe                 bool
	}{
		{"unchanged automatic output", "Ref", marked("previous release"), &snapshot{Text: marked("previous release")}, true},
		{"wiki typo correction", "Ref", "Documentation", &snapshot{Text: "Documantation"}, false},
		{"older generated page restored", "Ref", marked("older release"), &snapshot{Text: marked("previous release")}, false},
		{"reviewed guide correction", "Guide", "reviewed", &snapshot{Text: "older"}, true},
		{"new correction after review", "Guide", "later correction", &snapshot{Text: "older"}, false},
		{"no automatic baseline", "Ref", "unknown", nil, false},
		{"correction already merged into release", "Ref", "new release", &snapshot{Text: "older"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := safeToPublish(config, tc.title, snapshot{Text: tc.current}, marked("new release"), tc.published); got != tc.safe {
				t.Fatalf("safe=%v; want %v", got, tc.safe)
			}
		})
	}
	if !safeToPublish(config, "New", snapshot{Missing: true}, "new", nil) {
		t.Fatal("new page blocked")
	}
	if _, valid := unmark(strings.TrimRight(marked("body"), "\n")); !valid {
		t.Fatal("newline normalization invalidated generated page")
	}
}

func TestPublisherUsesRevisionAndCreationGuards(t *testing.T) {
	for _, missing := range []bool{false, true} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				t.Error(err)
			}
			if r.Method != http.MethodPost || r.Form.Get("action") != "edit" || r.Form.Get("starttimestamp") != "2026-10-18T00:00:00Z" {
				t.Error("edit lost write/time guards")
			}
			if missing {
				if r.Form.Get("createonly") != "1" || r.Form.Get("baserevid") != "" {
					t.Error("new page can overwrite an existing page")
				}
			} else if r.Form.Get("baserevid") != "42" || r.Form.Get("nocreate") != "1" {
				t.Error("existing page can overwrite a concurrent change or recreate a deleted page")
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"error":{"code":"editconflict","info":"A newer edit exists"}}`))
		}))
		client, err := mwclient.New(server.URL, "test")
		if err != nil {
			t.Fatal(err)
		}
		values := editParameters("Cli/guide/Test", "replacement", "summary", "2026-10-18T00:00:00Z", snapshot{Missing: missing, Revision: 42})
		values["token"] = "fixture"
		if err := client.Edit(values); err == nil || !strings.Contains(err.Error(), "editconflict") {
			t.Fatalf("concurrent edit conflict ignored: %v", err)
		}
		server.Close()
	}
}

func TestCheckExportsOnlyWikiCorrectionsAndPublishDoesNotLogin(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll("docs", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	item := document{Title: "Cli/guide/Test", Wikitext: filepath.Join(outputDir, "test.wiki")}
	if err := os.WriteFile(item.Wikitext, []byte(marked("Entirely new release guide")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(filepath.Join(outputDir, "pages.json"), []document{item}); err != nil {
		t.Fatal(err)
	}
	published := snapshot{Revision: 41, Text: marked("Documantation"), User: "Publisher", Comment: publicationSummary + " (v1.0.0)"}
	current := snapshot{Revision: 42, Text: strings.Replace(published.Text, "Documantation", "Documentation", 1), User: "Editor", Comment: "Fix spelling"}
	posts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			posts++
			t.Error("unsafe publish reached a write/login request")
		}
		revisions := []snapshot{current}
		if r.URL.Query().Get("revids") == "41" {
			revisions = []snapshot{published}
		} else if r.URL.Query().Get("rvlimit") != "" {
			revisions = append(revisions, published)
		}
		replyRevisions(w, item.Title, revisions, "")
	}))
	defer server.Close()
	config := configuration{API: server.URL, ReferenceRoot: "Cli/ref"}
	if err := synchronize(config, "publish"); err == nil || !strings.Contains(err.Error(), "publishing blocked") {
		t.Fatalf("wiki correction not protected: %v", err)
	}
	if posts != 0 {
		t.Fatal("publish wrote before completing preflight")
	}
	diff, err := os.ReadFile(filepath.Join(outputDir, "wiki", "000.wiki.diff"))
	if err != nil || !strings.Contains(string(diff), "-Documantation") || !strings.Contains(string(diff), "+Documentation") || strings.Contains(string(diff), "Entirely new release guide") || strings.Contains(string(diff), markerPrefix) {
		t.Fatalf("community diff includes release churn or lost correction: %v\n%s", err, diff)
	}
	preview, err := os.ReadFile(filepath.Join(outputDir, "wiki", "000.publish.diff"))
	if err != nil || !strings.Contains(string(preview), "Entirely new release guide") {
		t.Fatalf("full publication preview lost: %v", err)
	}
	if err := acceptReview(config, item.Title); err != nil {
		t.Fatal(err)
	}
	var updated configuration
	if err := readJSON(configPath, &updated); err != nil {
		t.Fatal(err)
	}
	if !safeToPublish(updated, item.Title, current, "new", &published) || safeToPublish(updated, item.Title, snapshot{Text: "Later correction"}, "new", &published) {
		t.Fatal("acceptance did not pin the reviewed content")
	}
}

func replyRevisions(w http.ResponseWriter, title string, revisions []snapshot, continuation string) {
	records := []interface{}{}
	for _, revision := range revisions {
		records = append(records, map[string]interface{}{"revid": revision.Revision, "user": revision.User, "comment": revision.Comment, "slots": map[string]interface{}{"main": map[string]interface{}{"content": revision.Text}}})
	}
	response := map[string]interface{}{"curtimestamp": "2026-10-18T00:00:00Z", "query": map[string]interface{}{"pages": []interface{}{map[string]interface{}{"title": title, "revisions": records}}}}
	if continuation != "" {
		response["continue"] = map[string]string{"continue": "||", "rvcontinue": continuation}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func TestFetchNormalizesReferenceTitlesAndRejectsHiddenContent(t *testing.T) {
	for _, hidden := range []bool{false, true} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			content := map[string]interface{}{"content": "reference", "texthidden": hidden}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"curtimestamp": "2026-10-18T00:00:00Z", "query": map[string]interface{}{"pages": []interface{}{map[string]interface{}{"title": "Cli/ref/mw dev", "revisions": []interface{}{map[string]interface{}{"revid": 42, "slots": map[string]interface{}{"main": content}}}}}}})
		}))
		client, err := mwclient.New(server.URL, "test")
		if err != nil {
			t.Fatal(err)
		}
		state, err := fetchPages(client, []string{"Cli/ref/mw_dev"})
		if hidden && err == nil {
			t.Fatal("unreadable content treated as writable")
		}
		if !hidden && (err != nil || state.Pages["Cli/ref/mw_dev"].Revision != 42) {
			t.Fatalf("title normalization failed: %v", err)
		}
		server.Close()
	}
}

func TestGuideLinksAreConvertedWithoutChangingCode(t *testing.T) {
	if _, err := osexec.LookPath("pandoc"); err != nil {
		t.Skip("pandoc is installed in the documentation CI job")
	}
	markdown := "[Install](../installation.md)\n\n```sh\necho '[Install](../installation.md)'\n```\n"
	converted, err := convertMarkdown([]byte(markdown), "advanced/test.md", map[string]string{"installation.md": "https://www.mediawiki.org/wiki/Cli/guide/Installation"})
	if err != nil {
		t.Fatal(err)
	}
	output := string(converted)
	if !strings.Contains(output, "https://www.mediawiki.org/wiki/Cli/guide/Installation") || !strings.Contains(output, "echo '[Install](../installation.md)'") {
		t.Fatalf("link rewrite changed code or lost link: %s", output)
	}
	if _, err := convertMarkdown([]byte("[Missing](missing.md)"), "index.md", nil); err == nil {
		t.Fatal("unmapped guide link accepted")
	}
}

func TestLastPublicationPaginatesPastManualBotEdits(t *testing.T) {
	config := configuration{ReferenceRoot: "Cli/ref", LegacyPublisher: "Publisher", LegacySummary: "Generated"}
	original := snapshot{Revision: 10, Text: "original reference", User: "Publisher", Comment: "Generated"}
	current := snapshot{Revision: 12, Text: "correction", User: "Editor", Comment: "Fix spelling"}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("revids") == "10" {
			replyRevisions(w, "Cli/ref/mw", []snapshot{original}, "")
			return
		}
		if r.URL.Query().Get("rvstartid") != "12" {
			t.Error("history is not bounded to the checked revision")
		}
		if r.URL.Query().Get("rvcontinue") == "older" {
			replyRevisions(w, "Cli/ref/mw", []snapshot{original}, "")
		} else {
			manual := snapshot{Revision: 11, Text: "manual bot edit", User: "Publisher", Comment: "Manual correction"}
			replyRevisions(w, "Cli/ref/mw", []snapshot{current, manual}, "older")
		}
	}))
	defer server.Close()
	client, err := mwclient.New(server.URL, "test")
	if err != nil {
		t.Fatal(err)
	}
	found, err := lastPublication(client, config, "Cli/ref/mw", current)
	if err != nil || found == nil || found.Revision != 10 || calls != 3 {
		t.Fatalf("last publication=%v, %v (%d calls)", found, err, calls)
	}
	// Reverts to that exact content need no review, while spelling edits do.
	if safeToPublish(config, "Cli/ref/mw", current, "new release", found) || !safeToPublish(config, "Cli/ref/mw", snapshot{Text: original.Text}, "new release", found) {
		t.Fatal("revert/correction classification incorrect")
	}
}

func TestReferenceRedirectKeepsDirectiveFirst(t *testing.T) {
	text := "#REDIRECT [[Cli/ref/wm]]\n"
	rendered := marked(text)
	if !strings.HasPrefix(rendered, text) {
		t.Fatal("metadata moved the redirect directive")
	}
	body, valid := unmark(rendered)
	if !valid || body != text {
		t.Fatal("redirect did not retain its fingerprint")
	}
	if _, valid := unmark(strings.Replace(rendered, "Cli/ref/wm", "Other", 1)); valid {
		t.Fatal("redirect target can change without detection")
	}
}

func TestPublishPreflightsLogsInAndSendsGuardedEdit(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("DOCS_UPDATE_USER", "writer")
	t.Setenv("DOCS_UPDATE_PASS", "fixture")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	item := document{Title: "Cli/guide/Test", Wikitext: filepath.Join(outputDir, "test.wiki")}
	desired := marked("= Release guide =\n")
	if err := os.WriteFile(item.Wikitext, []byte(desired), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(filepath.Join(outputDir, "pages.json"), []document{item}); err != nil {
		t.Fatal(err)
	}
	edits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Form.Get("prop") == "revisions|pageprops":
			_, _ = w.Write([]byte(`{"curtimestamp":"2026-10-18T00:00:00Z","query":{"pages":[{"title":"Cli/guide/Test","missing":true}]}}`))
		case r.Form.Get("meta") == "tokens":
			_, _ = w.Write([]byte(`{"query":{"tokens":{"logintoken":"login-fixture","csrftoken":"csrf-fixture"}}}`))
		case r.Form.Get("action") == "login":
			if r.Form.Get("lgname") != "writer" || r.Form.Get("lgpassword") != "fixture" {
				t.Error("credentials were not supplied through the login request")
			}
			_, _ = w.Write([]byte(`{"login":{"result":"Success"}}`))
		case r.Form.Get("action") == "edit":
			edits++
			if r.Form.Get("createonly") != "1" || r.Form.Get("text") != desired || r.Form.Get("assert") != "user" || r.Form.Get("token") != "csrf-fixture" {
				t.Error("guarded authenticated edit was not sent")
			}
			_, _ = w.Write([]byte(`{"edit":{"result":"Success"}}`))
		default:
			t.Error("unexpected API request")
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()
	if err := synchronize(configuration{API: server.URL, ReferenceRoot: "Cli/ref"}, "publish"); err != nil {
		t.Fatal(err)
	}
	if edits != 1 {
		t.Fatalf("sent %d edits; want one", edits)
	}
}

func TestGuideSourcesCannotEscapeOrOverwriteProjectDocumentation(t *testing.T) {
	for _, source := range []string{"../README.md", "/tmp/guide.md", "README.md", "guides/../index.md", "guides/example.txt"} {
		if _, err := guidePath(source); err == nil {
			t.Fatalf("unsafe guide path accepted: %s", source)
		}
	}
}

func TestWikiImportsResolveInternalLinks(t *testing.T) {
	if _, err := osexec.LookPath("pandoc"); err != nil {
		t.Skip("pandoc is installed in the documentation CI job")
	}
	config := configuration{PageURL: "https://www.mediawiki.org/wiki", Guides: []guide{{"guides/install.md", "Cli/guide/Installation"}}}
	result, err := convertWiki([]byte("[[Cli/guide/Installation|Install]] and [[Help:Editing|Help]]"), "index.md", config)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result), "guides/install.md") || !strings.Contains(string(result), "https://www.mediawiki.org/wiki/Help:Editing") || strings.Contains(string(result), `"wikilink"`) {
		t.Fatalf("import left broken wiki-relative links: %s", result)
	}
}

func TestUnpublishedGuideUsesItsReviewedImportRevision(t *testing.T) {
	original := snapshot{Revision: 40, Text: "Documantation", User: "Editor"}
	current := snapshot{Revision: 42, Text: "Documentation", User: "Editor", Comment: "Fix spelling"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("revids") == "40" {
			replyRevisions(w, "Guide", []snapshot{original}, "")
		} else {
			replyRevisions(w, "Guide", []snapshot{current}, "")
		}
	}))
	defer server.Close()
	client, err := mwclient.New(server.URL, "test")
	if err != nil {
		t.Fatal(err)
	}
	config := configuration{Reviewed: map[string]baseline{"Guide": {40, digest(normalize(original.Text))}}}
	baseline, err := lastPublication(client, config, "Guide", current)
	if err != nil || baseline == nil || baseline.Revision != 40 || baseline.Text != original.Text {
		t.Fatalf("import baseline=%v, %v", baseline, err)
	}
	if safeToPublish(config, "Guide", current, marked("different new release"), baseline) {
		t.Fatal("imported guide's wiki correction can be overwritten")
	}
	if !safeToPublish(config, "Guide", current, marked(current.Text), baseline) {
		t.Fatal("wiki correction already merged into source still blocked")
	}
}
