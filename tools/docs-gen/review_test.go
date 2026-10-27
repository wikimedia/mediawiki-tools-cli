package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRenderWikiPreviewRequestsModuleConfiguration(t *testing.T) {
	const title = "Cli/ref/mw_config"
	const text = "[[Cli/ref/mw|mw]]"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		if r.Method != http.MethodPost || r.Form.Get("action") != "parse" || r.Form.Get("title") != title || r.Form.Get("text") != text {
			t.Errorf("unexpected preview request: %s %v", r.Method, r.Form)
			http.Error(w, "expected read-only parse request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		props := "|" + r.Form.Get("prop") + "|"
		if strings.Contains(props, "|modules|") && !strings.Contains(props, "|jsconfigvars|") && !strings.Contains(props, "|encodedjsconfigvars|") {
			// Match the MediaWiki warning that previously broke previews.
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"warnings": map[string]interface{}{"parse": map[string]string{
					"warnings": `Property "modules" was set but not "jsconfigvars" or "encodedjsconfigvars". Configuration variables are necessary for proper module usage.`,
				}},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"parse": map[string]interface{}{
				"text":         "<p>Rendered wiki content</p>",
				"modulestyles": []string{"mediawiki.skinning.content"},
				"modules":      []string{"preview.script"},
				"jsconfigvars": map[string]string{"wgPageName": title},
			},
		})
	}))
	defer server.Close()

	rendered, err := renderWikiPreview(configuration{API: server.URL + "/w/api.php", PageURL: server.URL + "/wiki"}, title, text)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered, "<p>Rendered wiki content</p>") || !strings.Contains(rendered, "mediawiki.skinning.content") || !strings.Contains(rendered, "only=styles") {
		t.Fatalf("missing rendered content or styles: %s", rendered)
	}
	if strings.Contains(rendered, "preview.script") || strings.Contains(rendered, "wgPageName") {
		t.Fatal("preview unexpectedly included executable wiki module configuration")
	}
}

func TestPublicationPlanResolvesRelativeReferenceLinks(t *testing.T) {
	server := reviewServer{session: &reviewSession{ID: "fixture", Pages: []reviewItem{
		{Title: "Cli/ref/mw", Safe: true, Changed: true, Proposed: "[[../mw_api|mw api]] [[../mw_api#Options|Options]]"},
		{Title: "Cli/ref/mw_api", Safe: true, Changed: true, Current: snapshot{Missing: true}},
	}}}
	plan, err := server.selectPlan(selectedPages{Session: "fixture", IDs: []int{0}})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Warnings) != 1 || !strings.Contains(plan.Warnings[0], "Cli/ref/mw_api") {
		t.Fatalf("missing or duplicated relative-link warning: %v", plan.Warnings)
	}
	plan, err = server.selectPlan(selectedPages{Session: "fixture", IDs: []int{0, 1}})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Warnings) != 0 {
		t.Fatalf("selected new page still produces link warning: %v", plan.Warnings)
	}
}

func TestPublicationLineChanges(t *testing.T) {
	oldVersion := documentationVersion{Notice: true, Label: "branch / unreleased"}
	newVersion := documentationVersion{Notice: true, Label: "v0.34.0"}
	for _, tc := range []struct {
		name     string
		current  snapshot
		proposed string
		want     lineChanges
	}{
		{"additions", snapshot{Text: marked("one\n")}, marked("one\ntwo\n"), lineChanges{Added: 1}},
		{"removals", snapshot{Text: marked("one\ntwo\n")}, marked("one\n"), lineChanges{Removed: 1}},
		{"both", snapshot{Text: marked("old\n")}, marked("new\n"), lineChanges{Added: 1, Removed: 1}},
		{"empty existing page", snapshot{}, marked("new\n"), lineChanges{Added: 1}},
		{"remove all content", snapshot{Text: marked("old\n")}, marked(""), lineChanges{Removed: 1}},
		{"new page stays separate", snapshot{Missing: true}, marked("new\n"), lineChanges{}},
		{"tracking marker only", snapshot{Text: "same\n"}, marked("same\n"), lineChanges{}},
		{"version only", snapshot{Text: marked(versionedDocumentation("same\n", oldVersion))}, marked(versionedDocumentation("same\n", newVersion)), lineChanges{}},
		{"addition with new version", snapshot{Text: marked(versionedDocumentation("one\n", oldVersion))}, marked(versionedDocumentation("one\ntwo\n", newVersion)), lineChanges{Added: 1}},
		{"diff-like content", snapshot{Text: marked("--- old\n")}, marked("+++ new\n"), lineChanges{Added: 1, Removed: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := publicationLineChanges(tc.current, tc.proposed)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}
