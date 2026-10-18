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
	config := configuration{ReferenceRoot: "Cli/ref", LegacyPublisher: "Publisher", LegacySummary: "Generated", Reviewed: map[string]baseline{"Cli/guide/Install": {42, digest(normalize("reviewed"))}}}
	for _, tc := range []struct {
		name, title string
		page        snapshot
		safe        bool
	}{
		{"new page", "Cli/guide/New", snapshot{Missing: true}, true},
		{"unchanged generated page", "Cli/guide/New", snapshot{Text: marked("previous release")}, true},
		{"edited generated page", "Cli/guide/New", snapshot{Text: strings.Replace(marked("previous release"), "previous release", "wiki correction", 1)}, false},
		{"reviewed guide", "Cli/guide/Install", snapshot{Text: "reviewed\n"}, true},
		{"edited reviewed guide", "Cli/guide/Install", snapshot{Text: "new correction"}, false},
		{"legacy bot reference", "Cli/ref/mw", snapshot{Text: "previous", User: "Publisher", Comment: "Generated"}, true},
		{"legacy human correction", "Cli/ref/mw", snapshot{Text: "corrected", User: "Editor", Comment: "Fixed help"}, false},
		{"bot guide is not implicitly adopted", "Cli/guide/Install", snapshot{Text: "other", User: "Publisher", Comment: "Generated"}, false},
		{"broken marker cannot use legacy bypass", "Cli/ref/mw", snapshot{Text: markerPrefix + "bad -->\nchanged", User: "Publisher", Comment: "Generated"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := safeToPublish(config, tc.title, tc.page, marked("new release")); got != tc.safe {
				t.Fatalf("safe=%v; want %v", got, tc.safe)
			}
		})
	}
	// MediaWiki can strip trailing newlines; this must not produce false conflicts.
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

func TestCheckExportsWikiCorrectionsAndPublishDoesNotLogin(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll("docs", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	item := document{Title: "Cli/guide/Test", Wikitext: filepath.Join(outputDir, "test.wiki")}
	if err := os.WriteFile(item.Wikitext, []byte(marked("repo text")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(filepath.Join(outputDir, "pages.json"), []document{item}); err != nil {
		t.Fatal(err)
	}
	posts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			posts++
			t.Error("unsafe publish reached a write/login request")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"curtimestamp":"2026-10-18T00:00:00Z","query":{"pages":[{"title":"Cli/guide/Test","revisions":[{"revid":42,"user":"Editor","comment":"Fix","slots":{"main":{"content":"Community correction"}}}]}]}}`))
	}))
	defer server.Close()
	config := configuration{API: server.URL, ReferenceRoot: "Cli/ref"}
	if err := synchronize(config, "publish"); err == nil || !strings.Contains(err.Error(), "publishing blocked") {
		t.Fatalf("wiki correction not protected: %v", err)
	}
	if posts != 0 {
		t.Fatal("publish wrote before completing preflight")
	}
	diff, err := os.ReadFile(filepath.Join(outputDir, "wiki", "000.diff"))
	if err != nil || !strings.Contains(string(diff), "Community correction") || !strings.Contains(string(diff), "repo text") {
		t.Fatalf("missing review diff: %v", err)
	}
	if err := acceptReview(config, "Cli/guide/Test"); err != nil {
		t.Fatal(err)
	}
	var updated configuration
	if err := readJSON(configPath, &updated); err != nil {
		t.Fatal(err)
	}
	if !safeToPublish(updated, item.Title, snapshot{Text: "Community correction"}, "new") || safeToPublish(updated, item.Title, snapshot{Text: "Later correction"}, "new") {
		t.Fatal("acceptance did not pin the reviewed content")
	}
}

func TestFetchNormalizesReferenceTitlesAndRejectsHiddenContent(t *testing.T) {
	for _, hidden := range []bool{false, true} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			content := map[string]interface{}{"content": "reference", "texthidden": hidden}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"curtimestamp": "2026-10-18T00:00:00Z", "query": map[string]interface{}{"pages": []interface{}{map[string]interface{}{"title": "Cli/ref/mw docker", "revisions": []interface{}{map[string]interface{}{"revid": 42, "slots": map[string]interface{}{"main": content}}}}}}})
		}))
		client, err := mwclient.New(server.URL, "test")
		if err != nil {
			t.Fatal(err)
		}
		state, err := fetchPages(client, []string{"Cli/ref/mw_docker"})
		if hidden && err == nil {
			t.Fatal("unreadable content treated as writable")
		}
		if !hidden && (err != nil || state.Pages["Cli/ref/mw_docker"].Revision != 42) {
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

func TestLegacyRevertRequiresIdenticalPublishedContent(t *testing.T) {
	for _, tc := range []struct {
		text, comment string
		want          bool
	}{
		{"original reference", "Generated", true},
		{"human correction", "Generated", false},
		{"original reference", "Manual bot edit", false},
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("rvuser") != "Publisher" {
				t.Error("history query did not select the trusted publisher")
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"query": map[string]interface{}{"pages": []interface{}{map[string]interface{}{"title": "Cli/ref/mw", "revisions": []interface{}{map[string]interface{}{"user": "Publisher", "comment": tc.comment, "slots": map[string]interface{}{"main": map[string]interface{}{"content": "original reference"}}}}}}}})
		}))
		client, err := mwclient.New(server.URL, "test")
		if err != nil {
			t.Fatal(err)
		}
		config := configuration{ReferenceRoot: "Cli/ref", LegacyPublisher: "Publisher", LegacySummary: "Generated"}
		got, err := unchangedLegacyReference(client, config, "Cli/ref/mw", snapshot{Text: tc.text, User: "Editor", Comment: "Reverted vandalism"})
		if err != nil || got != tc.want {
			t.Fatalf("legacy verification=%v, %v; want %v", got, err, tc.want)
		}
		server.Close()
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
		case r.Form.Get("prop") == "revisions":
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
