package wiki

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

type moveAPIFixture struct {
	t                                   *testing.T
	mu                                  sync.Mutex
	pages                               map[string]wikiMovePage
	redirects                           map[string]string
	csrfRequests                        int
	moveCalls, moves, posts             int
	failAt                              int
	createDestinationAfterMove          bool
	missingRedirect, talkError          bool
	wantTalk                            bool
	paginationRequests                  int
	wantBot, denyBot, denyBotAfterFirst bool
}

func (f *moveAPIFixture) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !strings.HasPrefix(r.UserAgent(), cli.HTTPUserAgent+" ") {
		f.t.Errorf("unexpected user agent: %q", r.UserAgent())
	}
	if err := r.ParseForm(); err != nil {
		f.t.Error(err)
	}
	if r.Method == http.MethodPost {
		f.posts++
	}
	w.Header().Set("Content-Type", "application/json")
	write := func(data interface{}) { _ = json.NewEncoder(w).Encode(data) }
	withAssociatedPage := func(page wikiMovePage) wikiMovePage {
		switch page.Namespace {
		case 0:
			page.AssociatedPage = "Talk:" + page.Title
		case 10:
			page.AssociatedPage = "Template talk:" + strings.TrimPrefix(page.Title, "Template:")
		}
		return page
	}
	if f.wantBot && r.Form.Get("action") != "login" && r.Form.Get("type") != "login" {
		if r.Form.Get("assert") != "bot" {
			f.t.Error("bot mode did not assert the bot right on every request after login")
		}
		if f.denyBot || (f.denyBotAfterFirst && f.moves > 0) {
			write(map[string]interface{}{"error": map[string]string{"code": "assertbotfailed", "info": "Bot right unavailable"}})
			return
		}
	}
	switch r.Form.Get("action") {
	case "login":
		if r.Form.Get("lgname") != "Name@bot" || r.Form.Get("lgpassword") != "fixture-secret" || r.Form.Get("lgtoken") != "login-token" {
			f.t.Error("incorrect login parameters")
		}
		http.SetCookie(w, &http.Cookie{Name: "wiki-session", Value: "authenticated", Path: "/"})
		write(map[string]interface{}{"login": map[string]string{"result": "Success"}})
	case "query":
		if r.Form.Get("meta") == "tokens" {
			if r.Form.Get("type") == "login" {
				write(map[string]interface{}{"query": map[string]interface{}{"tokens": map[string]string{"logintoken": "login-token"}}})
			} else {
				f.csrfRequests++
				if _, err := r.Cookie("wiki-session"); err != nil {
					f.t.Error("CSRF request lost authenticated session")
				}
				write(map[string]interface{}{"query": map[string]interface{}{"tokens": map[string]string{"csrftoken": "csrf-token"}}})
			}
			return
		}
		if r.Form.Get("redirects") == "1" {
			var redirects []map[string]string
			for _, title := range strings.Split(r.Form.Get("titles"), "|") {
				seen := map[string]bool{}
				for {
					title = moveTitleKey(title)
					to, ok := f.redirects[title]
					if !ok || seen[title] {
						break
					}
					seen[title] = true
					to, fragment, _ := strings.Cut(to, "#")
					to = moveTitleKey(to)
					redirects = append(redirects, map[string]string{"from": title, "to": to, "tofragment": fragment})
					title = to
				}
			}
			write(map[string]interface{}{"query": map[string]interface{}{"redirects": redirects}})
			return
		}
		if r.Form.Get("generator") == "allpages" {
			f.paginationRequests++
			var pages []wikiMovePage
			for _, page := range f.pages {
				prefix := page.Title
				if page.Namespace != 0 {
					_, prefix, _ = strings.Cut(prefix, ":")
				}
				if strings.HasPrefix(prefix, r.Form.Get("gapprefix")) && strconv.Itoa(page.Namespace) == r.Form.Get("gapnamespace") {
					pages = append(pages, withAssociatedPage(page))
				}
			}
			sort.Slice(pages, func(i, j int) bool { return pages[i].Title < pages[j].Title })
			response := map[string]interface{}{}
			if len(pages) > 2 && r.Form.Get("gapcontinue") == "" {
				pages = pages[:2]
				response["continue"] = map[string]string{"continue": "gapcontinue||", "gapcontinue": "next"}
			} else if r.Form.Get("gapcontinue") == "next" {
				if r.Form.Get("continue") != "gapcontinue||" {
					f.t.Error("lost generic continuation parameter")
				}
				pages = pages[2:]
			}
			response["query"] = map[string]interface{}{"pages": pages}
			write(response)
			return
		}
		var pages []wikiMovePage
		var normalized []map[string]string
		for _, raw := range strings.Split(r.Form.Get("titles"), "|") {
			title := moveTitleKey(raw)
			page, ok := f.pages[title]
			if !ok {
				page = wikiMovePage{Title: title, Missing: true}
				if strings.HasPrefix(title, "Template:") {
					page.Namespace = 10
				}
			}
			if f.createDestinationAfterMove && f.moves == 1 && title == "Cli/ref/mw dev a" {
				page.ID, page.Missing = 999, false
			}
			pages = append(pages, withAssociatedPage(page))
			if raw != title {
				normalized = append(normalized, map[string]string{"from": raw, "to": title})
			}
		}
		write(map[string]interface{}{"query": map[string]interface{}{"pages": pages, "normalized": normalized}})
	case "move":
		f.moveCalls++
		assert := "user"
		if f.wantBot {
			assert = "bot"
		}
		if r.Method != http.MethodPost || r.Form.Get("token") != "csrf-token" || r.Form.Get("assert") != assert {
			f.t.Error("move lacked authenticated POST and CSRF token")
		}
		if _, err := r.Cookie("wiki-session"); err != nil {
			f.t.Error("move lost login session")
		}
		if r.Form.Has("bot") || r.Form.Has("noredirect") || r.Form.Has("ignorewarnings") || r.Form.Has("movesubpages") || r.Form.Get("watchlist") != "nochange" {
			f.t.Error("move changed redirect, warning, subpage or watchlist defaults")
		}
		if r.Form.Get("reason") != "Rename command docs" || (r.Form.Get("movetalk") == "1") != f.wantTalk {
			f.t.Error("incorrect move reason or talk-page flag")
		}
		if f.failAt == f.moveCalls {
			write(map[string]interface{}{"error": map[string]string{"code": "permissiondenied", "info": "Move denied by fixture"}})
			return
		}
		id, _ := strconv.ParseInt(r.Form.Get("fromid"), 10, 64)
		for title, page := range f.pages {
			if page.ID != id {
				continue
			}
			to := r.Form.Get("to")
			if _, ok := f.pages[to]; ok {
				f.t.Error("attempted to overwrite an existing destination")
			}
			f.moves++
			if f.redirects == nil {
				f.redirects = map[string]string{}
			}
			if page.Redirect {
				f.redirects[to] = f.redirects[title]
			}
			page.Title = to
			f.pages[to] = page
			f.pages[title] = wikiMovePage{Title: title, Namespace: page.Namespace, ID: 1000 + id, Redirect: !f.missingRedirect}
			if !f.missingRedirect {
				f.redirects[title] = to
			}
			result := map[string]interface{}{"from": title, "to": to, "redirectcreated": !f.missingRedirect}
			if f.talkError {
				result["talkmove-errors"] = []map[string]string{{"code": "articleexists", "info": "Talk target exists"}}
			}
			write(map[string]interface{}{"move": result})
			return
		}
		f.t.Errorf("unknown source page ID: %d", id)
		http.Error(w, "unknown source ID", http.StatusBadRequest)
	default:
		f.t.Errorf("unexpected API action: %s", r.Form.Get("action"))
		http.Error(w, "unexpected action", http.StatusBadRequest)
	}
}

func TestWikiPageMove(t *testing.T) {
	for _, tc := range []struct {
		name, wantError          string
		flags                    []string
		dryRun, collision        bool
		failAt, wantCalls, moves int
		changeAfterFirst         bool
		noRedirect, talkError    bool
		bot, denyBotAfterFirst   bool
		limit                    int
	}{
		{name: "prefix preserves suffixes and history", flags: []string{"--interval", "1ms"}, wantCalls: 3, moves: 3},
		{name: "public dry run never logs in", dryRun: true},
		{name: "bot mode asserts rights and leaves redirects", bot: true, wantCalls: 3, moves: 3},
		{name: "authenticated bot dry run", dryRun: true, bot: true},
		{name: "limit caps actual moves", limit: 2, wantCalls: 2, moves: 2},
		{name: "limit caps public dry run", limit: 1, dryRun: true},
		{name: "limit larger than matches", limit: 10, wantCalls: 3, moves: 3},
		{name: "collision outside limited batch does not block moves", limit: 2, collision: true, wantCalls: 2, moves: 2},
		{name: "losing bot right stops batch", bot: true, denyBotAfterFirst: true, wantCalls: 1, moves: 1, wantError: "assertbotfailed"},
		{name: "all destinations checked before first move", collision: true, wantError: "already exist"},
		{name: "dry run shows all collisions", dryRun: true, collision: true, wantError: "already exist"},
		{name: "stop on partial failure", failAt: 2, wantCalls: 2, moves: 1, wantError: "after 1 completed moves"},
		{name: "detect destination created during batch", changeAfterFirst: true, wantCalls: 1, moves: 1, wantError: "changed since planning"},
		{name: "missing redirect is not reported as success", noRedirect: true, wantCalls: 1, moves: 1, wantError: "no redirect was created"},
		{name: "talk failure reports already moved page", flags: []string{"--move-talk"}, talkError: true, wantCalls: 1, moves: 1, wantError: "content page moved"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &moveAPIFixture{t: t, pages: map[string]wikiMovePage{}, failAt: tc.failAt, createDestinationAfterMove: tc.changeAfterFirst, missingRedirect: tc.noRedirect, talkError: tc.talkError, wantTalk: tc.talkError}
			f.wantBot, f.denyBotAfterFirst = tc.bot, tc.denyBotAfterFirst
			for i, title := range []string{"Cli/ref/mw docker", "Cli/ref/mw docker a", "Cli/ref/mw docker b"} {
				f.pages[title] = wikiMovePage{Title: title, ID: int64(i + 1)}
			}
			if tc.collision {
				f.pages["Cli/ref/mw dev b"] = wikiMovePage{Title: "Cli/ref/mw dev b", ID: 100}
			}
			server := httptest.NewServer(http.HandlerFunc(f.serve))
			defer server.Close()
			var output bytes.Buffer
			cmd := NewWikiCmd()
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			cmd.SilenceErrors, cmd.SilenceUsage = true, true
			args := []string{"page", "move", "--prefix", "Cli/ref/mw_docker", "Cli/ref/mw_dev", "--wiki", server.URL + "/w/api.php", "--reason", "Rename command docs"}
			if tc.bot {
				args = append(args, "--bot")
			}
			if tc.limit > 0 {
				args = append(args, "--limit", strconv.Itoa(tc.limit))
			}
			if tc.dryRun {
				args = append(args, "--dry-run")
			}
			if !tc.dryRun || tc.bot {
				password := filepath.Join(t.TempDir(), "password")
				if err := os.WriteFile(password, []byte("fixture-secret\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				args = append(args, "--user", "Name@bot", "--password-file", password)
			}
			cmd.SetArgs(append(args, tc.flags...))
			err := cmd.Execute()
			if tc.wantError == "" && err != nil {
				t.Fatal(err)
			}
			if tc.wantError != "" && (err == nil || !strings.Contains(err.Error(), tc.wantError)) {
				t.Fatalf("error = %v, want %q", err, tc.wantError)
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.moveCalls != tc.wantCalls || f.moves != tc.moves {
				t.Fatalf("move calls=%d, successful moves=%d; want %d and %d", f.moveCalls, f.moves, tc.wantCalls, tc.moves)
			}
			if f.paginationRequests != 2 {
				t.Fatalf("prefix pagination was not followed: %d requests", f.paginationRequests)
			}
			if tc.dryRun && !tc.bot && f.posts != 0 {
				t.Fatalf("public dry run made %d POST requests", f.posts)
			}
			if strings.Contains(output.String(), "fixture-secret") {
				t.Fatal("password leaked into output")
			}
			wantPlan := 3
			if tc.limit > 0 {
				wantPlan = min(wantPlan, tc.limit)
			}
			if strings.Count(output.String(), "Would move") != wantPlan {
				t.Fatalf("complete plan was not displayed: %s", &output)
			}
			if tc.collision && wantPlan == 3 && !strings.Contains(output.String(), "BLOCKED") {
				t.Fatalf("collision missing from plan: %s", &output)
			}
		})
	}
}

func TestWikiPageMoveBotRightRequiredBeforePlanning(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		t.Run(fmt.Sprintf("dry-run=%t", dryRun), func(t *testing.T) {
			f := &moveAPIFixture{t: t, wantBot: true, denyBot: true}
			server := httptest.NewServer(http.HandlerFunc(f.serve))
			defer server.Close()
			var output bytes.Buffer
			cmd := NewWikiCmd()
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			cmd.SilenceErrors, cmd.SilenceUsage = true, true
			args := []string{"page", "move", "Old", "New", "--bot", "--wiki", server.URL + "/w/api.php", "--user", "Name@bot", "--password", "fixture-secret"}
			if dryRun {
				args = append(args, "--dry-run")
			}
			cmd.SetArgs(args)
			if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "assertbotfailed") {
				t.Fatalf("error = %v, want assertbotfailed", err)
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.moveCalls != 0 || f.paginationRequests != 0 || strings.Contains(output.String(), "Would move") {
				t.Fatalf("bot-right failure did not stop before planning: %s", &output)
			}
		})
	}
}

func TestWikiPageMoveResumesLimitedBatches(t *testing.T) {
	f := &moveAPIFixture{t: t, pages: map[string]wikiMovePage{}}
	for i, title := range []string{"Cli/ref/mw docker", "Cli/ref/mw docker a", "Cli/ref/mw docker b"} {
		f.pages[title] = wikiMovePage{Title: title, ID: int64(i + 1)}
	}
	server := httptest.NewServer(http.HandlerFunc(f.serve))
	defer server.Close()
	run := func(dryRun bool) string {
		t.Helper()
		var output bytes.Buffer
		cmd := NewWikiCmd()
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		args := []string{"page", "move", "Cli/ref/mw_docker", "Cli/ref/mw_dev", "--prefix", "--limit", "1", "--wiki", server.URL + "/w/api.php", "--reason", "Rename command docs"}
		if dryRun {
			args = append(args, "--dry-run")
		} else {
			args = append(args, "--user", "Name@bot", "--password", "fixture-secret")
		}
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		return output.String()
	}
	for batch := 0; batch < 4; batch++ {
		if batch == 1 {
			output := run(true)
			if strings.Count(output, "Skipping ") != 1 || strings.Count(output, "Would move ") != 1 || !strings.Contains(output, `Would move "Cli/ref/mw docker a"`) {
				t.Fatalf("dry run did not advance past completed move: %s", output)
			}
		}
		output := run(false)
		if strings.Count(output, "Skipping ") != batch {
			t.Fatalf("batch %d did not skip its completed predecessors: %s", batch, output)
		}
		wantMoves := 1
		if batch == 3 {
			wantMoves = 0
			if !strings.Contains(output, "No pages to move") {
				t.Fatalf("completed batch did not report completion: %s", output)
			}
		}
		if strings.Count(output, "Moved \"") != wantMoves {
			t.Fatalf("batch %d moved wrong number of pages: %s", batch, output)
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.moves != 3 || f.moveCalls != 3 || f.csrfRequests != 3 || f.posts != 7 {
		t.Fatalf("unexpected writes while resuming: moves=%d, calls=%d, CSRF requests=%d, POSTs=%d", f.moves, f.moveCalls, f.csrfRequests, f.posts)
	}
	for i, suffix := range []string{"", " a", " b"} {
		old, target := f.pages["Cli/ref/mw docker"+suffix], f.pages["Cli/ref/mw dev"+suffix]
		if !old.Redirect || target.ID != int64(i+1) || target.Redirect {
			t.Fatalf("source redirect or destination history lost: old=%+v, destination=%+v", old, target)
		}
	}
}

func TestWikiPageMoveSkipRequiresExactExistingTarget(t *testing.T) {
	for _, tc := range []struct {
		name, redirect, wantError            string
		sourceRedirect, missingTarget, chain bool
		wantSkipped, wantMoves               int
	}{
		{name: "direct redirect", sourceRedirect: true, redirect: "New title", wantSkipped: 1},
		{name: "underscores normalized", sourceRedirect: true, redirect: "New_title", wantSkipped: 1},
		{name: "ordinary page is a conflict", wantError: "already exist"},
		{name: "redirect elsewhere is a conflict", sourceRedirect: true, redirect: "Other", wantError: "already exist"},
		{name: "redirect chain is not a completed move", sourceRedirect: true, redirect: "Other", chain: true, wantError: "already exist"},
		{name: "section redirect is a conflict", sourceRedirect: true, redirect: "New title#Section", wantError: "already exist"},
		{name: "broken redirect is not a completed move", sourceRedirect: true, redirect: "New title", missingTarget: true, wantError: "missing destination"},
		{name: "unrelated redirect can itself be moved", sourceRedirect: true, redirect: "Other", missingTarget: true, wantMoves: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &moveAPIFixture{t: t, pages: map[string]wikiMovePage{"Old title": {Title: "Old title", ID: 1, Redirect: tc.sourceRedirect}}, redirects: map[string]string{"Old title": tc.redirect}}
			if !tc.missingTarget {
				f.pages["New title"] = wikiMovePage{Title: "New title", ID: 2}
			}
			if tc.chain {
				f.pages["Other"] = wikiMovePage{Title: "Other", ID: 3, Redirect: true}
				f.redirects["Other"] = "New title"
			}
			server := httptest.NewServer(http.HandlerFunc(f.serve))
			defer server.Close()
			var output bytes.Buffer
			cmd := NewWikiCmd()
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			cmd.SilenceErrors, cmd.SilenceUsage = true, true
			cmd.SetArgs([]string{"page", "move", "Old_title", "New_title", "--wiki", server.URL + "/w/api.php", "--user", "Name@bot", "--password", "fixture-secret", "--reason", "Rename command docs"})
			err := cmd.Execute()
			if tc.wantError == "" && err != nil {
				t.Fatal(err)
			}
			if tc.wantError != "" && (err == nil || !strings.Contains(err.Error(), tc.wantError)) {
				t.Fatalf("error=%v, want %q", err, tc.wantError)
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.moves != tc.wantMoves || f.moveCalls != tc.wantMoves || strings.Count(output.String(), "Skipping ") != tc.wantSkipped {
				t.Fatalf("wrong move/skip results: moves=%d calls=%d output=%s", f.moves, f.moveCalls, &output)
			}
		})
	}
}

func TestWikiPageMoveResumeChecksTalkPages(t *testing.T) {
	for _, state := range []string{"absent", "moved", "pending", "wrong redirect"} {
		t.Run(state, func(t *testing.T) {
			f := &moveAPIFixture{t: t, wantTalk: true, pages: map[string]wikiMovePage{
				"Old": {Title: "Old", ID: 1, Redirect: true},
				"New": {Title: "New", ID: 2},
			}, redirects: map[string]string{"Old": "New"}}
			if state != "absent" {
				f.pages["Talk:Old"] = wikiMovePage{Title: "Talk:Old", Namespace: 1, ID: 3, Redirect: state != "pending"}
			}
			if state == "moved" || state == "wrong redirect" {
				f.pages["Talk:New"] = wikiMovePage{Title: "Talk:New", Namespace: 1, ID: 4}
				f.redirects["Talk:Old"] = "Talk:New"
				if state == "wrong redirect" {
					f.redirects["Talk:Old"] = "Talk:Other"
				}
			}
			server := httptest.NewServer(http.HandlerFunc(f.serve))
			defer server.Close()
			var output bytes.Buffer
			cmd := NewWikiCmd()
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			cmd.SilenceErrors, cmd.SilenceUsage = true, true
			cmd.SetArgs([]string{"page", "move", "Old", "New", "--move-talk", "--wiki", server.URL + "/w/api.php", "--user", "Name@bot", "--password", "fixture-secret"})
			err := cmd.Execute()
			if state == "pending" || state == "wrong redirect" {
				if err == nil || !strings.Contains(err.Error(), "talk page \"Talk:Old\" is not moved") {
					t.Fatalf("error=%v, want outstanding talk-page move", err)
				}
			} else if err != nil || !strings.Contains(output.String(), "Skipping ") {
				t.Fatalf("completed talk page not skipped: error=%v output=%s", err, &output)
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.moveCalls != 0 || f.csrfRequests != 0 {
				t.Fatal("already-moved content was moved again")
			}
		})
	}
}

func TestWikiPageMoveLimitAfterFullBatchOfSkips(t *testing.T) {
	f := &moveAPIFixture{t: t, pages: map[string]wikiMovePage{}, redirects: map[string]string{}}
	for i := 0; i < 55; i++ {
		from, to := fmt.Sprintf("Old%03d", i), fmt.Sprintf("New%03d", i)
		f.pages[from] = wikiMovePage{Title: from, ID: int64(i + 1), Redirect: i < 50}
		if i < 50 {
			f.pages[to] = wikiMovePage{Title: to, ID: int64(100 + i)}
			f.redirects[from] = to
		}
	}
	server := httptest.NewServer(http.HandlerFunc(f.serve))
	defer server.Close()
	var output bytes.Buffer
	cmd := NewWikiCmd()
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"page", "move", "Old", "New", "--prefix", "--limit", "2", "--wiki", server.URL + "/w/api.php", "--user", "Name@bot", "--password", "fixture-secret", "--reason", "Rename command docs"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.moves != 2 || strings.Count(output.String(), "Skipping ") != 50 || f.pages["New050"].ID != 51 || f.pages["New051"].ID != 52 {
		t.Fatalf("limit did not advance beyond completed batch: moves=%d output=%s", f.moves, &output)
	}
	if _, ok := f.pages["New052"]; ok {
		t.Fatal("limit exceeded")
	}
}

func TestWikiPageMoveSingleAndNamespaces(t *testing.T) {
	for _, prefix := range []bool{false, true} {
		t.Run(fmt.Sprintf("prefix=%t", prefix), func(t *testing.T) {
			f := &moveAPIFixture{t: t, pages: map[string]wikiMovePage{"Template:Old": {Title: "Template:Old", Namespace: 10, ID: 42}}}
			server := httptest.NewServer(http.HandlerFunc(f.serve))
			defer server.Close()
			var output bytes.Buffer
			cmd := NewWikiCmd()
			cmd.SetOut(&output)
			args := []string{"page", "move", "Template:Old", "Template:New", "--wiki", server.URL + "/w/api.php", "--user", "Name@bot", "--password", "fixture-secret", "--reason", "Rename command docs"}
			if prefix {
				args = append(args, "--prefix")
			}
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(output.String(), `Moved "Template:Old" -> "Template:New" (redirect left)`) {
				t.Fatalf("missing successful move: %s", &output)
			}
		})
	}
}

func TestWikiPageMoveRejectsInvalidRequestsBeforeContactingWiki(t *testing.T) {
	for _, args := range [][]string{
		{"Old", "New"},
		{"Old", "New", "--anon"},
		{"", "New", "--dry-run"},
		{"Old#Section", "New", "--dry-run"},
		{"Old|Other", "New", "--dry-run"},
		{"Old", "New", "--dry-run", "--title", "Ignored"},
		{"Old", "New", "--dry-run", "--interval=-1s"},
		{"Old", "New", "--dry-run", "--limit=-1"},
		{"Old", "New", "--dry-run", "--bot"},
		{"Old", "New", "--dry-run", "--bot", "--anon", "--user", "Name@bot", "--password", "fixture-secret"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			cmd := NewWikiCmd()
			cmd.SilenceErrors, cmd.SilenceUsage = true, true
			cmd.SetArgs(append([]string{"page", "move", "--wiki", "http://invalid.invalid/w/api.php"}, args...))
			if err := cmd.Execute(); err == nil {
				t.Fatal("invalid move accepted")
			}
		})
	}
}

func TestWikiPageMoveRejectsUnsafePlans(t *testing.T) {
	for _, tc := range []struct {
		from, to, wantError string
		prefix              bool
	}{
		{from: "Old_title", to: "Old title", wantError: "same wiki title"},
		{from: "Old", to: "Old/sub", prefix: true, wantError: "must not overlap"},
		{from: "Old/sub", to: "Old", prefix: true, wantError: "must not overlap"},
		{from: "Missing", to: "New", wantError: "no pages found"},
		{from: "Missing", to: "New", prefix: true, wantError: "no pages found"},
	} {
		t.Run(tc.from+" to "+tc.to, func(t *testing.T) {
			f := &moveAPIFixture{t: t, pages: map[string]wikiMovePage{}}
			server := httptest.NewServer(http.HandlerFunc(f.serve))
			defer server.Close()
			cmd := NewWikiCmd()
			cmd.SilenceErrors, cmd.SilenceUsage = true, true
			args := []string{"page", "move", tc.from, tc.to, "--wiki", server.URL + "/w/api.php", "--dry-run"}
			if tc.prefix {
				args = append(args, "--prefix")
			}
			cmd.SetArgs(args)
			if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("error = %v, want %q", err, tc.wantError)
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.posts != 0 {
				t.Fatalf("unsafe plan made %d POST requests", f.posts)
			}
		})
	}
}
