package wiki

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mcpserver"
)

func TestPageInfoMCPMatchesAnonymousCLIJSON(t *testing.T) {
	isolateWikiProfiles(t)
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	http.DefaultTransport = wikiRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Host != "www.mediawiki.org" ||
			req.URL.Path != "/w/api.php" || req.URL.Query().Get("titles") != "Earth" ||
			req.URL.Query().Get("redirects") != "1" || req.Header.Get("Authorization") != "" || req.Header.Get("Cookie") != "" {
			t.Errorf("unexpected page info request %s %s", req.Method, req.URL)
		}
		const body = `{"query":{"pages":[{"pageid":1,"lastrevid":2,"title":"Earth","fullurl":"https://www.mediawiki.org/wiki/Earth","protection":[]}]}}`
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	stdout, err := runWikiProfile(t, []string{"page", "info", "Earth", "--anon", "--wiki", "www.mediawiki.org", "--follow-redirects", "--output", "json"}, "")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server := mcpserver.New("test-server")
	pageInfoTool().Register(server)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "wiki_page_info",
		Arguments: map[string]any{"title": "Earth", "followRedirects": true},
	})
	if err != nil || result.IsError {
		t.Fatalf("page info: %+v, %v", result, err)
	}
	var cliJSON any
	if err := json.Unmarshal([]byte(stdout), &cliJSON); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cliJSON, result.StructuredContent) {
		t.Fatalf("CLI and MCP JSON differ: CLI %s, MCP %+v", stdout, result.StructuredContent)
	}
}

func TestPageReadCommandsOutputAndValidation(t *testing.T) {
	isolateWikiProfiles(t)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		q := r.URL.Query()
		if r.Method != "GET" || q.Get("action") != "query" {
			t.Error("page read made a non-read request")
		}
		switch q.Get("prop") {
		case "info":
			if q.Get("redirects") != "1" {
				t.Error(q)
			}
			fmt.Fprint(w, `{"query":{"pages":[{"pageid":1,"ns":0,"title":"Earth","fullurl":"https://example.invalid/wiki/Earth"}]}}`)
		case "revisions":
			if q.Get("rvdir") != "newer" || q.Get("rvlimit") != "2" {
				t.Error(q)
			}
			fmt.Fprint(w, `{"query":{"pages":[{"pageid":1,"title":"Earth","revisions":[{"revid":1,"parentid":0,"timestamp":"2026-01-01T00:00:00Z","userhidden":true,"commenthidden":true,"size":12},{"revid":2,"parentid":1,"timestamp":"2026-01-02T00:00:00Z","user":"Fixture","comment":"Edit","size":13}]}]}}`)
		case "categories":
			if q.Get("clshow") != "" {
				t.Error(q)
			}
			fmt.Fprint(w, `{"query":{"pages":[{"pageid":1,"title":"Earth","categories":[{"ns":14,"title":"Category:Physics","sortkeyprefix":"Earth"},{"ns":14,"title":"Category:Science","hidden":true}]}]}}`)
		default:
			t.Error(q)
		}
	}))
	defer server.Close()
	common := []string{"--anon", "--wiki", server.URL + "/w/api.php"}
	run := func(args []string) (string, error) {
		return runWikiProfile(t, append(append([]string{"page"}, args...), common...), "")
	}
	if out, err := run([]string{"info", "Earth", "--follow-redirects", "--output", "jq", "--format", ".fullurl"}); err != nil || strings.TrimSpace(out) != "https://example.invalid/wiki/Earth" {
		t.Fatalf("info: %q %v", out, err)
	}
	if out, err := run([]string{"history", "Earth", "--order", "newer", "--limit", "2", "--output", "table"}); err != nil || !strings.Contains(out, "REVISION") || !strings.Contains(out, "[hidden]") {
		t.Fatalf("history table: %q %v", out, err)
	}
	if out, err := run([]string{"history", "Earth", "--order", "newer", "--limit", "2", "--output", "json"}); err != nil || !strings.Contains(out, `"userhidden":true`) {
		t.Fatalf("history JSON: %q %v", out, err)
	}
	if out, err := run([]string{"categories", "Earth", "--output", "titles"}); err != nil || out != "Category:Physics\nCategory:Science\n" {
		t.Fatalf("categories: %q %v", out, err)
	}
	before := calls
	for _, args := range [][]string{{"history", "Earth", "--start", "yesterday"}, {"history", "Earth", "--order", "wrong"}, {"history", "Earth", "--limit", "-1"}, {"history", "Earth", "--by-user", "One", "--exclude-user", "Two"}, {"categories", "Earth", "--hidden", "wrong"}, {"info", ""}, {"categories", "Earth", "--output", "titles", "--format", ".title"}} {
		if _, err := run(args); err == nil {
			t.Errorf("invalid page read accepted: %v", args)
		}
	}
	if calls != before {
		t.Fatal("invalid read parameters contacted wiki")
	}
}
