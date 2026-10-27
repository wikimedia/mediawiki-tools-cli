package main

import (
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	mwclient "cgt.name/pkg/go-mwclient"
	"cgt.name/pkg/go-mwclient/params"
)

//go:embed review.html
var reviewHTML []byte

type reviewItem struct {
	ID        int         `json:"id"`
	Title     string      `json:"title"`
	Source    string      `json:"source"`
	Area      string      `json:"area"`
	URL       string      `json:"url"`
	Status    string      `json:"status"`
	Safe      bool        `json:"safe"`
	WikiEdits bool        `json:"wikiEdits"`
	Revision  int64       `json:"revision"`
	Changed   bool        `json:"changed"`
	Changes   lineChanges `json:"changes"`
	Current   snapshot    `json:"-"`
	Baseline  *snapshot   `json:"-"`
	Proposed  string      `json:"-"`
	Markdown  string      `json:"-"`
}

type lineChanges struct {
	Added   int `json:"added"`
	Removed int `json:"removed"`
}

type reviewSession struct {
	ID         string               `json:"id"`
	Version    documentationVersion `json:"version"`
	Pages      []reviewItem         `json:"pages"`
	Timestamp  string               `json:"timestamp"`
	CanPublish bool                 `json:"canPublish"`
	Stale      bool                 `json:"stale"`
}

type reviewServer struct {
	mu      sync.Mutex
	config  configuration
	session *reviewSession
	token   string
	origin  string
}

type selectedPages struct {
	Session string `json:"session"`
	IDs     []int  `json:"ids"`
}

type publicationPlan struct {
	Session  string   `json:"session"`
	IDs      []int    `json:"ids"`
	Titles   []string `json:"titles"`
	Version  string   `json:"version"`
	Warnings []string `json:"warnings"`
}

func randomReviewToken() (string, error) {
	value := make([]byte, 24)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func documentationArea(config configuration, doc document) string {
	if strings.HasPrefix(doc.Title, config.ReferenceRoot+"/") {
		command := strings.Split(strings.TrimPrefix(doc.Title, config.ReferenceRoot+"/"), "_")
		if len(command) > 1 {
			return "Commands / " + command[1]
		}
		return "Commands / Overview"
	}
	for _, guide := range config.Guides {
		if guide.Title == doc.Title {
			if strings.HasPrefix(guide.Source, "guides/dev/") {
				return "Guides / Development"
			}
			if strings.HasPrefix(guide.Source, "guides/gitlab/") {
				return "Guides / GitLab"
			}
			return "Guides / Getting started"
		}
	}
	return "Commands / Overview"
}

func loadReviewSession(config configuration) (*reviewSession, error) {
	// Reuse the publisher's read-only inspection, baselines and on-disk reports.
	if err := synchronize(config, "review"); err != nil {
		return nil, err
	}
	var state review
	if err := readJSON(filepath.Join(outputDir, "wiki", "review.json"), &state); err != nil {
		return nil, err
	}
	var documents []document
	if err := readJSON(filepath.Join(outputDir, "pages.json"), &documents); err != nil {
		return nil, err
	}
	var version documentationVersion
	if err := readJSON(filepath.Join(outputDir, "version.json"), &version); err != nil {
		return nil, fmt.Errorf("run make docs to generate version information: %w", err)
	}
	id, err := randomReviewToken()
	if err != nil {
		return nil, err
	}
	session := &reviewSession{ID: id, Version: version, Timestamp: state.Timestamp, CanPublish: os.Getenv("DOCS_UPDATE_USER") != "" && os.Getenv("DOCS_UPDATE_PASS") != ""}
	for index, doc := range documents {
		proposed, err := os.ReadFile(doc.Wikitext)
		if err != nil {
			return nil, err
		}
		markdown, err := os.ReadFile(doc.Markdown)
		if err != nil {
			return nil, err
		}
		current, ok := state.Pages[doc.Title]
		if !ok {
			return nil, fmt.Errorf("missing reviewed snapshot for %s", doc.Title)
		}
		item := reviewItem{ID: index, Title: doc.Title, Source: doc.Source, Area: documentationArea(config, doc), URL: wikiURL(config, doc.Title), Current: current, Proposed: string(proposed), Markdown: string(markdown), Revision: current.Revision}
		if baseline, ok := state.Baselines[doc.Title]; ok {
			item.Baseline = &baseline
			item.WikiEdits = documentationBody(baseline.Text) != documentationBody(current.Text)
		}
		item.Safe = !current.Translation && safeToPublish(config, doc.Title, current, item.Proposed, item.Baseline)
		item.Changed = current.Missing || normalize(current.Text) != normalize(item.Proposed)
		item.Changes, err = publicationLineChanges(current, item.Proposed)
		if err != nil {
			return nil, fmt.Errorf("classify changes for %s: %w", doc.Title, err)
		}
		switch {
		case !item.Safe:
			item.Status = "Needs review"
		case current.Missing:
			item.Status = "New"
		case !item.Changed:
			item.Status = "Unchanged"
		case normalize(withoutVersionNotice(documentationBody(current.Text))) == normalize(withoutVersionNotice(documentationBody(item.Proposed))):
			item.Status = "Version only"
		default:
			item.Status = "Updated"
		}
		session.Pages = append(session.Pages, item)
	}
	return session, nil
}

func publicationLineChanges(current snapshot, proposed string) (lineChanges, error) {
	var changes lineChanges
	// New pages have their own filter. Ignore publication metadata so version
	// labels and content fingerprints do not turn additions into mixed changes.
	if current.Missing {
		return changes, nil
	}
	before := strings.TrimRight(withoutVersionNotice(documentationBody(current.Text)), "\r\n")
	after := strings.TrimRight(withoutVersionNotice(documentationBody(proposed)), "\r\n")
	if before != "" {
		before = normalize(before)
	}
	if after != "" {
		after = normalize(after)
	}
	diff, err := unifiedReviewDiff(before, after)
	if err != nil {
		return changes, err
	}
	inHunk := false
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "@@ ") {
			inHunk = true
			continue
		}
		if !inHunk || line == "" {
			continue
		}
		switch line[0] {
		case '+':
			changes.Added++
		case '-':
			changes.Removed++
		}
	}
	return changes, nil
}

func unifiedReviewDiff(before, after string) (string, error) {
	if before == after {
		return "No changes.\n", nil
	}
	dir, err := os.MkdirTemp("", "mwcli-docs-diff-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	for name, text := range map[string]string{"current.wiki": before, "proposed.wiki": after} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
			return "", err
		}
	}
	cmd := exec.Command("git", "--no-pager", "diff", "--no-index", "--no-ext-diff", "--no-color", "--", "current.wiki", "proposed.wiki")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	var failure *exec.ExitError
	if err != nil && (!errors.As(err, &failure) || failure.ExitCode() != 1) {
		return "", fmt.Errorf("generate diff: %w: %s", err, output)
	}
	return string(output), nil
}

func (s *reviewServer) selectPlan(request selectedPages) (publicationPlan, error) {
	plan := publicationPlan{Session: request.Session, Version: s.session.Version.Label, Warnings: []string{}}
	if request.Session != s.session.ID || s.session.Stale {
		return plan, fmt.Errorf("this review is out of date; refresh before publishing")
	}
	if len(request.IDs) == 0 {
		return plan, fmt.Errorf("select at least one page")
	}
	selected := map[string]bool{}
	for _, id := range request.IDs {
		if id < 0 || id >= len(s.session.Pages) {
			return plan, fmt.Errorf("unknown page %d", id)
		}
		item := s.session.Pages[id]
		if selected[item.Title] {
			return plan, fmt.Errorf("duplicate selected page: %s", item.Title)
		}
		if !item.Safe {
			return plan, fmt.Errorf("%s has wiki edits that need review", item.Title)
		}
		if !item.Changed {
			return plan, fmt.Errorf("%s is already up to date", item.Title)
		}
		selected[item.Title] = true
		plan.IDs = append(plan.IDs, id)
		plan.Titles = append(plan.Titles, item.Title)
	}
	missing := map[string]bool{}
	for _, item := range s.session.Pages {
		if item.Current.Missing && !selected[item.Title] {
			missing[normalizedWikiTitle(item.Title)] = true
		}
	}
	links := regexp.MustCompile(`\[\[([^\]|#]+)`)
	warned := map[string]bool{}
	for _, id := range plan.IDs {
		item := s.session.Pages[id]
		for _, match := range links.FindAllStringSubmatch(item.Proposed, -1) {
			target := strings.TrimPrefix(match[1], ":")
			if strings.HasPrefix(target, "../") || strings.HasPrefix(target, "./") {
				target = path.Clean(path.Join(item.Title, target))
			}
			key := normalizedWikiTitle(target)
			if missing[key] && !warned[key] {
				plan.Warnings = append(plan.Warnings, item.Title+" links to an unpublished, unselected page: "+target)
				warned[key] = true
			}
		}
	}
	sort.Strings(plan.Warnings)
	return plan, nil
}

func normalizedWikiTitle(title string) string {
	return strings.ReplaceAll(strings.TrimSpace(title), "_", " ")
}

func (s *reviewServer) publish(plan publicationPlan) ([]string, error) {
	// Fetch only selected pages: unrelated conflicts do not block this publication.
	client, err := newWiki(s.config)
	if err != nil {
		return nil, err
	}
	state, err := fetchPages(client, plan.Titles)
	if err != nil {
		return nil, err
	}
	for _, id := range plan.IDs {
		item := s.session.Pages[id]
		latest := state.Pages[item.Title]
		if latest.Missing != item.Current.Missing || latest.Revision != item.Revision || latest.Text != item.Current.Text {
			return nil, fmt.Errorf("%s changed on the wiki; refresh and review it again", item.Title)
		}
	}
	username, password := os.Getenv("DOCS_UPDATE_USER"), os.Getenv("DOCS_UPDATE_PASS")
	if username == "" || password == "" {
		return nil, fmt.Errorf("start docs-review with DOCS_UPDATE_USER and DOCS_UPDATE_PASS set to publish")
	}
	if err := client.Login(username, password); err != nil {
		return nil, err
	}
	client.Assert = mwclient.AssertUser
	// After an attempted edit, require a fresh review even if the outcome is uncertain.
	s.session.Stale = true
	published := []string{}
	for _, id := range plan.IDs {
		item := s.session.Pages[id]
		err := client.Edit(editParameters(item.Title, item.Proposed, publicationSummary+" ("+plan.Version+")", state.Timestamp, item.Current))
		if err != nil && !errors.Is(err, mwclient.ErrEditNoChange) {
			return published, fmt.Errorf("publishing %s: %w; refresh to inspect any completed edits", item.Title, err)
		}
		published = append(published, item.Title)
	}
	return published, nil
}

func renderWikiPreview(config configuration, title, text string) (string, error) {
	client, err := newWiki(config)
	if err != nil {
		return "", err
	}
	// MediaWiki requires configuration alongside modules, even though this static
	// preview only consumes module styles and does not execute wiki JavaScript.
	reply, err := client.Post(params.Values{"action": "parse", "formatversion": "2", "title": title, "text": text, "contentmodel": "wikitext", "prop": "text|modules|jsconfigvars", "preview": "1", "disableeditsection": "1", "useskin": "vector-2022"})
	if err != nil {
		return "", err
	}
	data, err := reply.Marshal()
	if err != nil {
		return "", err
	}
	var result struct {
		Parse struct {
			Text   string   `json:"text"`
			Styles []string `json:"modulestyles"`
		} `json:"parse"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	if result.Parse.Text == "" {
		return "", fmt.Errorf("wiki returned no preview HTML")
	}
	endpoint, err := url.Parse(config.API)
	if err != nil {
		return "", err
	}
	endpoint.Path = path.Join(path.Dir(endpoint.Path), "load.php")
	styles := append([]string{"site.styles"}, result.Parse.Styles...)
	endpoint.RawQuery = url.Values{"lang": {"en"}, "modules": {strings.Join(styles, "|")}, "only": {"styles"}, "skin": {"vector-2022"}}.Encode()
	return `<!doctype html><html lang="en"><head><meta charset="utf-8"><base href="` + html.EscapeString(config.PageURL) + `/"><link rel="stylesheet" href="` + html.EscapeString(endpoint.String()) + `"><style>body{font:16px/1.6 system-ui,sans-serif;color:#202122;background:white;margin:24px;max-width:1000px}pre{white-space:pre-wrap;overflow-wrap:anywhere;background:#f8f9fa;padding:12px}a{color:#36c}table{border-collapse:collapse}td,th{padding:8px}h1,h2{line-height:1.3}img{max-width:100%}</style></head><body class="skin-vector"><h1>` + html.EscapeString(title) + `</h1><div class="mw-parser-output">` + result.Parse.Text + `</div></body></html>`, nil
}

func reviewJSON(w http.ResponseWriter, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		fmt.Fprintf(os.Stderr, "review response: %v\n", err)
	}
}

func (s *reviewServer) handleAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Header.Get("X-Docs-Review-Token") != s.token {
		http.Error(w, "invalid review token", http.StatusForbidden)
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" && origin != s.origin {
		http.Error(w, "invalid origin", http.StatusForbidden)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var err error
	switch r.URL.Path {
	case "/api/session":
		if r.Method != http.MethodGet {
			http.Error(w, "GET required", 405)
			return
		}
		reviewJSON(w, s.session)
		return
	case "/api/page":
		if r.Method != http.MethodGet {
			http.Error(w, "GET required", 405)
			return
		}
		id, parseErr := strconv.Atoi(r.URL.Query().Get("id"))
		if parseErr != nil || id < 0 || id >= len(s.session.Pages) {
			http.Error(w, "unknown page", 400)
			return
		}
		item := s.session.Pages[id]
		diff, diffErr := unifiedReviewDiff(documentationBody(item.Current.Text), documentationBody(item.Proposed))
		if diffErr != nil {
			err = diffErr
			break
		}
		wikiDiff := "No wiki corrections since the recorded baseline."
		if item.Baseline != nil {
			wikiDiff, err = unifiedReviewDiff(documentationBody(item.Baseline.Text), documentationBody(item.Current.Text))
			if err != nil {
				break
			}
		} else if !item.Current.Missing {
			wikiDiff = "No automatic publication baseline was found. Review the current wiki text against the proposal before recording acceptance."
		}
		reviewJSON(w, map[string]interface{}{"page": item, "diff": diff, "wikiDiff": wikiDiff, "source": item.Markdown, "current": documentationBody(item.Current.Text), "proposed": documentationBody(item.Proposed)})
		return
	case "/api/preview":
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", 405)
			return
		}
		var request struct {
			ID      int    `json:"id"`
			Session string `json:"session"`
			Current bool   `json:"current"`
		}
		if err = json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&request); err != nil {
			break
		}
		if request.Session != s.session.ID || request.ID < 0 || request.ID >= len(s.session.Pages) {
			err = fmt.Errorf("review changed; reload the selected page")
			break
		}
		item := s.session.Pages[request.ID]
		body := item.Proposed
		if request.Current {
			body = item.Current.Text
		}
		rendered, renderErr := renderWikiPreview(s.config, item.Title, documentationBody(body))
		if renderErr != nil {
			err = renderErr
			break
		}
		reviewJSON(w, map[string]string{"html": rendered})
		return
	case "/api/plan", "/api/publish":
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", 405)
			return
		}
		var request selectedPages
		if err = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&request); err != nil {
			break
		}
		plan, planErr := s.selectPlan(request)
		if planErr != nil {
			err = planErr
			break
		}
		if r.URL.Path == "/api/plan" {
			reviewJSON(w, plan)
			return
		}
		published, publishErr := s.publish(plan)
		response := map[string]interface{}{"published": published, "stale": s.session.Stale}
		if publishErr != nil {
			response["error"] = publishErr.Error()
		}
		reviewJSON(w, response)
		return
	case "/api/accept":
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", 405)
			return
		}
		var request selectedPages
		if err = json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&request); err != nil {
			break
		}
		if request.Session != s.session.ID || s.session.Stale || len(request.IDs) != 1 || request.IDs[0] < 0 || request.IDs[0] >= len(s.session.Pages) {
			err = fmt.Errorf("refresh before accepting a wiki revision")
			break
		}
		item := &s.session.Pages[request.IDs[0]]
		if item.Current.Missing || item.Current.Translation {
			err = fmt.Errorf("only an existing source page can be marked reviewed")
			break
		}
		client, clientErr := newWiki(s.config)
		if clientErr != nil {
			err = clientErr
			break
		}
		latest, fetchErr := fetchPages(client, []string{item.Title})
		if fetchErr != nil {
			err = fetchErr
			break
		}
		if latest.Pages[item.Title].Revision != item.Revision {
			err = fmt.Errorf("page changed on the wiki; refresh first")
			break
		}
		config, loadErr := loadConfig()
		if loadErr != nil {
			err = loadErr
			break
		}
		if config.Reviewed == nil {
			config.Reviewed = map[string]baseline{}
		}
		config.Reviewed[item.Title] = baseline{item.Revision, digest(normalize(item.Current.Text))}
		if err = writeJSON(configPath, config); err != nil {
			break
		}
		s.config = config
		item.Safe = true
		if item.Changed {
			item.Status = "Updated"
		} else {
			item.Status = "Unchanged"
		}
		reviewJSON(w, s.session)
		return
	case "/api/refresh":
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", 405)
			return
		}
		config, loadErr := loadConfig()
		if loadErr != nil {
			err = loadErr
			break
		}
		s.session.Stale = true
		if err = generate(config); err != nil {
			break
		}
		session, loadErr := loadReviewSession(config)
		if loadErr != nil {
			err = loadErr
			break
		}
		s.config = config
		s.session = session
		reviewJSON(w, session)
		return
	default:
		http.NotFound(w, r)
		return
	}
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		reviewJSON(w, map[string]string{"error": err.Error()})
	}
}

func serveDocumentationReview(config configuration) error {
	fmt.Println("Reading wiki revisions and generating review diffs…")
	session, err := loadReviewSession(config)
	if err != nil {
		return err
	}
	token, err := randomReviewToken()
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	origin := "http://" + listener.Addr().String()
	review := &reviewServer{config: config, session: session, token: token, origin: origin}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/", review.handleAPI)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(reviewHTML)
	})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != listener.Addr().String() {
			http.Error(w, "invalid host", http.StatusForbidden)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline' https://www.mediawiki.org; img-src https: data:; font-src https: data:; frame-src 'self' about:; frame-ancestors 'none'; object-src 'none'; base-uri https://www.mediawiki.org")
		mux.ServeHTTP(w, r)
	})
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	address := origin + "/#" + token
	fmt.Printf("Review %d pages: %s\nPress Ctrl+C to stop.\n", len(session.Pages), address)
	err = server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
