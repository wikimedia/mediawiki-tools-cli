package wiki

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gitlab.wikimedia.org/repos/releng/cli/internal/accesspolicy"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mcpserver"
)

func TestPageInfoMCPMatchesPublicOnlyCLIJSON(t *testing.T) {
	isolateWikiProfiles(t)
	if err := config.PutKeyValueOnDisk(accesspolicy.ConfigKey, "true"); err != nil {
		t.Fatal(err)
	}
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
	stdout, err := runWikiProfile(t, []string{"page", "info", "Earth", "--username", "fixture", "--password", "fixture", "--wiki", "www.mediawiki.org", "--follow-redirects", "--output", "json"}, "")
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
