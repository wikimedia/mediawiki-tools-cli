package phabricator

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	mwconfig "gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mcpserver"
)

func TestPhabricatorMCPPublicSearch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	calls := 0
	httpClient := &http.Client{Transport: authTransport(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.Method != http.MethodGet || req.URL.Host != "phabricator.wikimedia.org" || req.URL.Path != "/maniphest/" ||
			req.Header.Get("Authorization") != "" || req.Header.Get("Cookie") != "" {
			t.Errorf("unexpected public search request %s %s", req.Method, req.URL)
		}
		for key, want := range map[string]string{
			"projects": "PHID-PROJ-ci", "priorities": "100,80", "assigned": "none()", "limit": "2", "statuses": "open()",
		} {
			if got := req.URL.Query().Get(key); got != want {
				t.Errorf("query %s = %q, want %q", key, got, want)
			}
		}
		return anonymousResponse(req, http.StatusOK, `<ul class="phui-oi-list-view"><li class="phui-oi">
			<a class="phui-oi-link" href="/T123">Public task</a>
		</li></ul>`), nil
	})}
	server := mcpserver.New("test-server")
	anonymousMCPSearch(anonymousPhabricator{http: httpClient}).Register(server)
	session := connectTestPhabricatorMCP(t, ctx, server)

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != 1 || !tools.Tools[0].Annotations.ReadOnlyHint {
		t.Fatalf("unexpected tools: %+v", tools.Tools)
	}
	toolName := tools.Tools[0].Name
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: toolName,
		Arguments: map[string]any{
			"projects": []string{"PHID-PROJ-ci"}, "priorities": []int{100, 80}, "unassigned": true, "limit": 2,
		},
	})
	if err != nil || result.IsError {
		t.Fatalf("search: %+v, %v", result, err)
	}
	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var search mcpSearchResult
	if err := json.Unmarshal(raw, &search); err != nil || len(search.Tasks) != 1 || search.Tasks[0].ID != "T123" {
		t.Fatalf("search result %s: %v", raw, err)
	}
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: toolName, Arguments: map[string]any{"limit": 0}})
	if err != nil || !result.IsError {
		t.Fatalf("invalid search: %+v, %v", result, err)
	}
	if calls != 1 {
		t.Fatalf("expected one HTTP request, got %d", calls)
	}
}

func connectTestPhabricatorMCP(t *testing.T, ctx context.Context, server *mcp.Server) *mcp.ClientSession {
	t.Helper()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

func TestPhabricatorMCPBindsNamedProfile(t *testing.T) {
	isolatedAuthConfig(t)
	for key, value := range map[string]string{
		"phabricator.default_profile":        "other",
		"phabricator.profiles.work.key":      "fixture-token",
		"phabricator.profiles.work.username": "example",
		"phabricator.profiles.work.url":      defaultPhabricatorURL,
		"phabricator.public_only":            "true",
	} {
		if err := mwconfig.PutKeyValueOnDisk(key, value); err != nil {
			t.Fatal(err)
		}
	}
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	http.DefaultTransport = authTransport(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/api/maniphest.search" {
			t.Errorf("unexpected profile request %s %s", req.Method, req.URL)
		}
		if err := req.ParseForm(); err != nil {
			return nil, err
		}
		var payload struct {
			Conduit     struct{ Token string } `json:"__conduit__"`
			Constraints struct {
				Query      string   `json:"query"`
				Priorities []int    `json:"priorities"`
				Assigned   []string `json:"assigned"`
			} `json:"constraints"`
		}
		if err := json.Unmarshal([]byte(req.FormValue("params")), &payload); err != nil {
			return nil, err
		}
		if payload.Conduit.Token != "fixture-token" {
			t.Error("request did not use the explicitly selected profile")
		}
		if payload.Constraints.Query != "example" || !reflect.DeepEqual(payload.Constraints.Priorities, []int{80}) ||
			!reflect.DeepEqual(payload.Constraints.Assigned, []string{"none()"}) {
			t.Errorf("unexpected constraints: %+v", payload.Constraints)
		}
		return pasteResponse(`{"result":` + accessPage(t, accessTask(123, "users", wikimediaPublicSpace)) + `}`), nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server := mcpserver.New("test-server")
	cmd := &cobra.Command{Use: "mcp"}
	cmd.SetContext(ctx)
	tools, err := MCPTools(cmd, "work")
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools {
		tool.Register(server)
	}
	session := connectTestPhabricatorMCP(t, ctx, server)
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "phabricator_search_tasks",
		Arguments: map[string]any{"query": "example", "priorities": []int{80}, "unassigned": true},
	})
	if err != nil || result.IsError {
		t.Fatalf("search: %+v, %v", result, err)
	}
	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var page mcpSearchResult
	if err := json.Unmarshal(raw, &page); err != nil || len(page.Tasks) != 1 || !page.Tasks[0].Redacted || page.Tasks[0].Title != nonPublicLabel {
		t.Fatalf("profile search did not apply the configured public-only policy: %s, %v", raw, err)
	}
}
