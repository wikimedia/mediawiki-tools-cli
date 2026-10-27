package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

const (
	moveOldTitle = "Cli/ref/mw_docker"
	moveNewTitle = "Cli/ref/mw_dev"
)

type publicationMoveFixture struct {
	t                                      *testing.T
	mu                                     sync.Mutex
	pages                                  map[string]snapshot
	history                                map[string][]snapshot
	events                                 []string
	posts, moves, edits                    int
	failEditOnce, denyMove, editDuringMove bool
}

func (f *publicationMoveFixture) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := r.ParseForm(); err != nil {
		f.t.Error(err)
	}
	if !strings.HasPrefix(r.UserAgent(), cli.HTTPUserAgent+" ") {
		f.t.Error("missing CLI user agent")
	}
	w.Header().Set("Content-Type", "application/json")
	write := func(value interface{}) { _ = json.NewEncoder(w).Encode(value) }
	if r.Method == http.MethodPost {
		f.posts++
	}
	revisionRecord := func(page snapshot) map[string]interface{} {
		return map[string]interface{}{"revid": page.Revision, "user": page.User, "comment": page.Comment, "slots": map[string]interface{}{"main": map[string]interface{}{"content": page.Text}}}
	}
	pageRecord := func(title string, page snapshot) map[string]interface{} {
		result := map[string]interface{}{"title": strings.ReplaceAll(title, "_", " "), "pageid": page.PageID, "missing": page.Missing}
		if !page.Missing {
			result["revisions"] = []interface{}{revisionRecord(page)}
		}
		if page.Translation {
			result["pageprops"] = map[string]string{"translate-is-translation": "1"}
		}
		return result
	}
	switch r.Form.Get("action") {
	case "query":
		if r.Form.Get("meta") == "tokens" {
			write(map[string]interface{}{"query": map[string]interface{}{"tokens": map[string]string{"logintoken": "login-token", "csrftoken": "csrf-token"}}})
			return
		}
		var records []interface{}
		if id := r.Form.Get("revids"); id != "" {
			for title, history := range f.history {
				for _, page := range history {
					if strconv.FormatInt(page.Revision, 10) == id {
						records = append(records, pageRecord(title, page))
						break
					}
				}
				if len(records) > 0 {
					break
				}
			}
		} else {
			for _, title := range strings.Split(r.Form.Get("titles"), "|") {
				page, ok := f.pages[title]
				if !ok {
					page = snapshot{Missing: true}
				}
				record := pageRecord(title, page)
				if r.Form.Get("rvlimit") != "" {
					var revisions []interface{}
					for _, revision := range f.history[title] {
						revisions = append(revisions, revisionRecord(revision))
					}
					record["revisions"] = revisions
				}
				records = append(records, record)
			}
		}
		write(map[string]interface{}{"curtimestamp": "2026-10-27T12:00:00Z", "query": map[string]interface{}{"pages": records}})
	case "login":
		if r.Form.Get("lgname") != "fixture-bot" || r.Form.Get("lgpassword") != "fixture-password" {
			f.t.Error("incorrect fixture credentials")
		}
		write(map[string]interface{}{"login": map[string]string{"result": "Success"}})
	case "move":
		if r.Method != http.MethodPost || r.Form.Get("assert") != "user" || r.Form.Get("token") != "csrf-token" || r.Form.Get("fromid") != "101" || r.Form.Get("to") != moveNewTitle {
			f.t.Error("move lost authenticated POST, CSRF or source ID")
		}
		if r.Form.Has("noredirect") || r.Form.Has("bot") || r.Form.Has("ignorewarnings") || r.Form.Has("movesubpages") || r.Form.Has("movetalk") || r.Form.Get("watchlist") != "nochange" {
			f.t.Error("move changed redirect or subpage/watchlist defaults")
		}
		if _, exists := f.pages[moveNewTitle]; exists {
			f.t.Error("move overwrote an existing destination")
		}
		if f.denyMove {
			write(map[string]interface{}{"error": map[string]string{"code": "cantmove", "info": "Move permission unavailable"}})
			return
		}
		source := f.pages[moveOldTitle]
		if f.editDuringMove {
			source.Text = marked("Concurrent correction")
		}
		source.Revision, source.Comment = 20, r.Form.Get("reason")
		f.pages[moveNewTitle] = source
		f.history[moveNewTitle] = append([]snapshot{source}, f.history[moveOldTitle]...)
		redirect := snapshot{PageID: 202, Revision: 21, Text: "#REDIRECT [[Cli/ref/mw dev]]\n", User: "fixture-bot", Comment: "Created redirect when moving page"}
		f.pages[moveOldTitle], f.history[moveOldTitle] = redirect, []snapshot{redirect}
		f.moves++
		f.events = append(f.events, "move")
		write(map[string]interface{}{"move": map[string]interface{}{"from": "Cli/ref/mw docker", "to": "Cli/ref/mw dev", "redirectcreated": true}})
	case "edit":
		if f.moves == 0 && f.pages[moveNewTitle].PageID != 101 {
			f.t.Error("created new content before transferring the old history")
		}
		if r.Form.Get("title") != moveNewTitle || r.Form.Get("baserevid") != "20" || r.Form.Get("nocreate") != "1" || r.Form.Has("createonly") || r.Form.Get("assert") != "user" {
			f.t.Error("edit did not guard the moved destination revision")
		}
		if f.failEditOnce {
			f.failEditOnce = false
			write(map[string]interface{}{"error": map[string]string{"code": "fixturefailure", "info": "Edit interrupted after move"}})
			return
		}
		page := f.pages[moveNewTitle]
		page.Revision, page.Text, page.Comment = 30, r.Form.Get("text"), r.Form.Get("summary")
		f.pages[moveNewTitle] = page
		f.history[moveNewTitle] = append([]snapshot{page}, f.history[moveNewTitle]...)
		f.edits++
		f.events = append(f.events, "edit")
		write(map[string]interface{}{"edit": map[string]interface{}{"result": "Success", "newrevid": 30}})
	default:
		f.t.Errorf("unexpected fixture action %q", r.Form.Get("action"))
		http.Error(w, "unexpected action", 400)
	}
}

func setupPublicationMove(t *testing.T) (*publicationMoveFixture, configuration) {
	t.Helper()
	t.Chdir(t.TempDir())
	t.Setenv("DOCS_UPDATE_USER", "fixture-bot")
	t.Setenv("DOCS_UPDATE_PASS", "fixture-password")
	if err := os.MkdirAll("docs", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	documents := []document{
		{Title: moveNewTitle, MoveFrom: moveOldTitle, Source: "commands/", Markdown: filepath.Join(outputDir, "new.md"), Wikitext: filepath.Join(outputDir, "new.wiki")},
		{Title: moveOldTitle, Source: "commands/", Markdown: filepath.Join(outputDir, "old.md"), Wikitext: filepath.Join(outputDir, "old.wiki")},
	}
	for _, doc := range documents {
		body := "New documentation\n"
		if doc.Title == moveOldTitle {
			body = "#REDIRECT [[" + moveNewTitle + "]]\n"
		}
		if err := os.WriteFile(doc.Wikitext, []byte(marked(body)), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(doc.Markdown, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := writeJSON(filepath.Join(outputDir, "pages.json"), documents); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(filepath.Join(outputDir, "version.json"), documentationVersion{Label: "fixture"}); err != nil {
		t.Fatal(err)
	}
	original := snapshot{PageID: 101, Revision: 10, Text: marked("Old documentation\n"), User: "Publisher", Comment: publicationSummary + " (v0.33.0)"}
	f := &publicationMoveFixture{t: t, pages: map[string]snapshot{moveOldTitle: original}, history: map[string][]snapshot{moveOldTitle: {original}}}
	server := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(server.Close)
	config := configuration{API: server.URL, PageURL: server.URL + "/wiki", ReferenceRoot: "Cli/ref"}
	if err := writeJSON(configPath, config); err != nil {
		t.Fatal(err)
	}
	return f, config
}

func TestDocumentationPublicationMovesBeforeEditingAndResumes(t *testing.T) {
	for _, interrupted := range []bool{false, true} {
		t.Run(fmt.Sprintf("interrupted=%t", interrupted), func(t *testing.T) {
			f, config := setupPublicationMove(t)
			f.failEditOnce = interrupted
			if err := synchronize(config, "check"); err != nil {
				t.Fatal(err)
			}
			if f.posts != 0 {
				t.Fatal("read-only check wrote to the wiki")
			}
			report, err := os.ReadFile(filepath.Join(outputDir, "wiki", "publish-report.md"))
			if err != nil || !strings.Contains(string(report), "MOVE from "+moveOldTitle) {
				t.Fatalf("move missing from CI report: %v %s", err, report)
			}
			err = synchronize(config, "publish")
			if interrupted {
				if err == nil || !strings.Contains(err.Error(), "fixturefailure") {
					t.Fatalf("expected interrupted edit: %v", err)
				}
				if err := synchronize(config, "publish"); err != nil {
					t.Fatal(err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			posts := f.posts
			if err := synchronize(config, "publish"); err != nil {
				t.Fatal(err)
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.moves != 1 || f.edits != 1 || f.posts != posts || strings.Join(f.events, ",") != "move,edit" {
				t.Fatalf("non-idempotent publication: moves=%d edits=%d events=%v", f.moves, f.edits, f.events)
			}
			if f.pages[moveNewTitle].PageID != 101 || documentationRedirect(f.pages[moveOldTitle].Text) != normalizedWikiTitle(moveNewTitle) {
				t.Fatal("publication lost original page identity or its redirect")
			}
		})
	}
}

func TestDocumentationMoveReviewPairsSelectionAndContentDiff(t *testing.T) {
	for _, selectSource := range []bool{false, true} {
		t.Run(fmt.Sprintf("select-source=%t", selectSource), func(t *testing.T) {
			f, config := setupPublicationMove(t)
			session, err := loadReviewSession(config)
			if err != nil {
				t.Fatal(err)
			}
			if len(session.Moves) != 1 || session.Pages[0].Status != "Move" || session.Pages[1].Status != "Redirect after move" || session.Pages[0].comparison().Text != f.pages[moveOldTitle].Text {
				t.Fatalf("review does not describe the move: %+v", session)
			}
			server := &reviewServer{config: config, session: session, token: "fixture"}
			request := httptest.NewRequest(http.MethodGet, "/api/page?id=0", nil)
			request.Header.Set("X-Docs-Review-Token", "fixture")
			response := httptest.NewRecorder()
			server.handleAPI(response, request)
			if response.Code != 200 || !strings.Contains(response.Body.String(), "-Old documentation") || !strings.Contains(response.Body.String(), "+New documentation") {
				t.Fatalf("move diff did not compare existing content: %s", response.Body)
			}
			id := 0
			if selectSource {
				id = 1
			}
			plan, err := server.selectPlan(selectedPages{Session: session.ID, IDs: []int{id}})
			if err != nil || len(plan.IDs) != 2 || len(plan.Moves) != 1 {
				t.Fatalf("selection did not include its move partner: %+v %v", plan, err)
			}
			if f.posts != 0 {
				t.Fatal("review planning wrote to the wiki")
			}
			published, err := server.publish(plan)
			if err != nil || len(published) != 2 || f.moves != 1 || f.edits != 1 {
				t.Fatalf("selective publication did not perform move and update: %v %v", published, err)
			}
		})
	}
}

func TestDocumentationMovesBlockUnsafeSourcesAndDestinations(t *testing.T) {
	for _, kind := range []string{"wiki correction", "occupied destination", "translation"} {
		t.Run(kind, func(t *testing.T) {
			f, config := setupPublicationMove(t)
			switch kind {
			case "wiki correction":
				page := f.pages[moveOldTitle]
				page.Revision, page.Text, page.Comment = 11, marked("Unreviewed wiki correction"), "Fix a typo"
				f.pages[moveOldTitle] = page
				f.history[moveOldTitle] = append([]snapshot{page}, f.history[moveOldTitle]...)
			case "occupied destination":
				page := f.pages[moveOldTitle]
				page.PageID = 999
				f.pages[moveNewTitle], f.history[moveNewTitle] = page, []snapshot{page}
			case "translation":
				page := f.pages[moveOldTitle]
				page.Translation = true
				f.pages[moveOldTitle] = page
			}
			session, err := loadReviewSession(config)
			if err != nil {
				t.Fatal(err)
			}
			if session.Pages[0].Safe || session.Pages[1].Safe {
				t.Fatal("unsafe move was marked publishable")
			}
			if err := synchronize(config, "publish"); err == nil || !strings.Contains(err.Error(), "publishing blocked") {
				t.Fatalf("unsafe publication allowed: %v", err)
			}
			if f.posts != 0 {
				t.Fatal("unsafe move reached login or a write")
			}
		})
	}
}

func TestDocumentationMoveDoesNotOverwriteConcurrentCorrections(t *testing.T) {
	f, config := setupPublicationMove(t)
	f.editDuringMove = true
	if err := synchronize(config, "publish"); err == nil || !strings.Contains(err.Error(), "content or redirect changed") {
		t.Fatalf("concurrent correction not protected: %v", err)
	}
	if f.moves != 1 || f.edits != 0 || !strings.Contains(f.pages[moveNewTitle].Text, "Concurrent correction") {
		t.Fatal("concurrent correction was overwritten")
	}
}

func TestDocumentationMovePermissionFailureDoesNotFallBackToCreatingPage(t *testing.T) {
	f, config := setupPublicationMove(t)
	f.denyMove = true
	if err := synchronize(config, "publish"); err == nil || !strings.Contains(err.Error(), "cantmove") {
		t.Fatalf("move permission error ignored: %v", err)
	}
	if f.moves != 0 || f.edits != 0 {
		t.Fatal("failed move fell back to editing")
	}
}

func TestDocumentationMoveReviewRejectsStaleSourceBeforeLogin(t *testing.T) {
	for _, title := range []string{moveOldTitle, moveNewTitle} {
		t.Run(title, func(t *testing.T) {
			f, config := setupPublicationMove(t)
			session, err := loadReviewSession(config)
			if err != nil {
				t.Fatal(err)
			}
			server := &reviewServer{config: config, session: session}
			plan, err := server.selectPlan(selectedPages{Session: session.ID, IDs: []int{0}})
			if err != nil {
				t.Fatal(err)
			}
			page := f.pages[moveOldTitle]
			page.Revision, page.Text = 11, marked("Later wiki correction")
			f.pages[title] = page
			if _, err := server.publish(plan); err == nil || !strings.Contains(err.Error(), "changed on the wiki") {
				t.Fatalf("stale move review allowed: %v", err)
			}
			if f.posts != 0 {
				t.Fatal("stale move reached login or writes")
			}
		})
	}
}

func TestDocumentationMoveMetadataAndRedirectGuards(t *testing.T) {
	documents := []document{{Title: moveNewTitle, MoveFrom: moveOldTitle}, {Title: moveOldTitle}}
	proposed := map[string]string{moveNewTitle: marked("new"), moveOldTitle: marked("#REDIRECT [[" + moveNewTitle + "]]\n")}
	state := review{Pages: map[string]snapshot{moveOldTitle: {PageID: 101, Text: "old"}, moveNewTitle: {Missing: true}}}
	for _, kind := range []string{"mismatched redirect", "duplicate source", "duplicate target"} {
		t.Run(kind, func(t *testing.T) {
			docs := append([]document(nil), documents...)
			bodies := map[string]string{}
			for title, body := range proposed {
				bodies[title] = body
			}
			switch kind {
			case "mismatched redirect":
				bodies[moveOldTitle] = marked("#REDIRECT [[Unrelated]]")
			case "duplicate source":
				docs = append(docs, document{Title: "Other", MoveFrom: moveOldTitle})
			case "duplicate target":
				docs = append(docs, docs[0])
			}
			if _, err := planDocumentationMoves(docs, bodies, state); err == nil {
				t.Fatal("ambiguous move metadata accepted")
			}
		})
	}
	for _, text := range []string{"#REDIRECT [[" + moveNewTitle + "#Section]]", "#REDIRECT [[" + moveNewTitle + "]]\nAdditional content"} {
		if documentationRedirect(text) != "" || samePublicationText(text, proposed[moveOldTitle]) {
			t.Fatalf("non-equivalent redirect treated as complete: %q", text)
		}
	}
}

func TestDocumentationMoveAcceptPinsSourceRevisionAndKeepsCollisionBlocked(t *testing.T) {
	for _, collision := range []bool{false, true} {
		t.Run(fmt.Sprintf("collision=%t", collision), func(t *testing.T) {
			f, config := setupPublicationMove(t)
			page := f.pages[moveOldTitle]
			page.Revision, page.Text, page.Comment = 11, marked("Wiki correction"), "Fix"
			f.pages[moveOldTitle] = page
			f.history[moveOldTitle] = append([]snapshot{page}, f.history[moveOldTitle]...)
			if collision {
				page.PageID = 999
				f.pages[moveNewTitle], f.history[moveNewTitle] = page, []snapshot{page}
			}
			session, err := loadReviewSession(config)
			if err != nil {
				t.Fatal(err)
			}
			server := &reviewServer{config: config, session: session, token: "fixture"}
			body, _ := json.Marshal(selectedPages{Session: session.ID, IDs: []int{0}})
			request := httptest.NewRequest(http.MethodPost, "/api/accept", bytes.NewReader(body))
			request.Header.Set("X-Docs-Review-Token", "fixture")
			response := httptest.NewRecorder()
			server.handleAPI(response, request)
			if collision {
				if response.Code == 200 || session.Pages[0].Safe || session.Pages[1].Safe {
					t.Fatal("acceptance bypassed a destination collision")
				}
			} else {
				var saved configuration
				if err := readJSON(configPath, &saved); err != nil {
					t.Fatal(err)
				}
				if response.Code != 200 || saved.Reviewed[moveOldTitle].Revision != 11 || !session.Pages[0].Safe || !session.Pages[1].Safe {
					t.Fatalf("acceptance did not pin the original source and approve its pair: %s", response.Body)
				}
			}
		})
	}
}
