package phabricator

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	mwconfig "gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mcpserver"
)

func TestPhabricatorMCPPublicSearch(t *testing.T) {
	isolatedAuthConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	calls := 0
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	http.DefaultTransport = authTransport(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.Method != http.MethodGet || req.URL.Host != "phabricator.wikimedia.org" || req.URL.Path != "/maniphest/" ||
			req.Header.Get("Authorization") != "" || req.Header.Get("Cookie") != "" {
			t.Errorf("unexpected public search request %s %s", req.Method, req.URL)
		}
		for key, want := range map[string]string{
			"projects": "PHID-PROJ-ci", "priorities": "100,80", "assigned": "none()", "authors": "Example",
			"limit": "50", "statuses": "open()", "order": "newest",
		} {
			if got := req.URL.Query().Get(key); got != want {
				t.Errorf("query %s = %q, want %q", key, got, want)
			}
		}
		return anonymousResponse(req, http.StatusOK, `<ul class="phui-oi-list-view"><li class="phui-oi">
			<a class="phui-oi-link" href="/T123">Public task</a>
		</li></ul><div class="phui-pager-view"><a href="/maniphest/?after=123">Next</a></div>
		<input name="projectPHIDs[]" value="PHID-PROJ-ci">
		<input name="priorities[]" value="100"><input name="priorities[]" value="80">
		<input name="assignedPHIDs[]" value="none()"><input name="authorPHIDs[]" value="PHID-USER-example">
		<input name="statuses[]" value="open()">`), nil
	})
	var stdout bytes.Buffer
	cmd := NewPhabricatorCmd()
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"task", "list", "--project", "PHID-PROJ-ci", "--priority", "100,80", "--unassigned", "--author", "Example", "--order", "newest", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if err := mwconfig.PutKeyValueOnDisk("phabricator.default_profile", "unused"); err != nil {
		t.Fatal(err)
	}
	if err := mwconfig.PutKeyValueOnDisk("phabricator.profiles.unused.key", "unused-token"); err != nil {
		t.Fatal(err)
	}
	server := mcpserver.New("test-server")
	operations, err := MCPTools(cmd, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range operations {
		operation.Register(server)
	}
	session := connectTestPhabricatorMCP(t, ctx, server)

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != 5 {
		t.Fatalf("unexpected tools: %+v", tools.Tools)
	}
	toolName := "phabricator_search_tasks"
	for _, tool := range tools.Tools {
		if !tool.Annotations.ReadOnlyHint || tool.Name == "phabricator_get_paste" {
			t.Fatalf("unexpected anonymous tool: %+v", tool)
		}
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: toolName,
		Arguments: map[string]any{
			"projects": []string{"PHID-PROJ-ci"}, "priorities": []int{100, 80}, "unassigned": true, "author": "Example", "order": "newest",
		},
	})
	if err != nil || result.IsError {
		t.Fatalf("search: %+v, %v", result, err)
	}
	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var search taskPage
	if err := json.Unmarshal(raw, &search); err != nil || len(search.Data) != 1 || search.Data[0].ID != 123 ||
		search.Cursor.After == nil || *search.Cursor.After != "123" || search.Cursor.Limit != 50 {
		t.Fatalf("search result %s: %v", raw, err)
	}
	var cliJSON, mcpJSON any
	if err := json.Unmarshal(stdout.Bytes(), &cliJSON); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &mcpJSON); err != nil || !reflect.DeepEqual(cliJSON, mcpJSON) {
		t.Fatalf("CLI and MCP JSON differ: CLI %s, MCP %s, error %v", stdout.Bytes(), raw, err)
	}
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: toolName, Arguments: map[string]any{"query": "test", "limit": 0}})
	if err != nil || !result.IsError {
		t.Fatalf("invalid search: %+v, %v", result, err)
	}
	if calls != 2 {
		t.Fatalf("expected two HTTP requests, got %d", calls)
	}
}

func TestPhabricatorMCPPublicTaskAndComments(t *testing.T) {
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	http.DefaultTransport = authTransport(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Host != "phabricator.wikimedia.org" || req.URL.Path != "/T123" ||
			req.Header.Get("Authorization") != "" || req.Header.Get("Cookie") != "" {
			t.Errorf("unexpected public task request %s %s", req.Method, req.URL)
		}
		return anonymousResponse(req, http.StatusOK, `
			<meta property="og:title" content="T123 Public task">
			<div class="phui-property-list-text-content"><div class="phabricator-remarkup">Task details</div></div>
			<div class="phui-timeline-event-view"><div class="phui-timeline-title"><a class="phui-link-person">Example</a></div>
			<div class="transaction-comment"><div class="phabricator-remarkup">A comment</div></div></div>`), nil
	})
	server := mcpserver.New("test-server")
	for _, operation := range phabricatorMCPTools(nil, false) {
		operation.Register(server)
	}
	session := connectTestPhabricatorMCP(t, t.Context(), server)
	for name, expected := range map[string]string{"phabricator_get_task": "Task details", "phabricator_get_comments": "A comment", "phabricator_get_task_comments": "A comment"} {
		result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: map[string]any{"taskId": "123"}})
		if err != nil || result.IsError {
			t.Fatalf("%s: %+v, %v", name, result, err)
		}
		raw, err := json.Marshal(result.StructuredContent)
		if err != nil || !bytes.Contains(raw, []byte(expected)) || (name == "phabricator_get_task" && !bytes.Contains(raw, []byte("A comment"))) {
			t.Fatalf("%s: %s, %v", name, raw, err)
		}
	}
	result, err := session.CallTool(t.Context(), &mcp.CallToolParams{
		Name: "phabricator_get_task", Arguments: map[string]any{"ref": "https://example.invalid/T123"},
	})
	if err != nil || !result.IsError {
		t.Fatalf("foreign task URL accepted: %+v, %v", result, err)
	}
}

func TestPhabricatorMCPPasteRequiresGrant(t *testing.T) {
	isolatedAuthConfig(t)
	cmd := &cobra.Command{Use: "mcp"}
	cmd.Flags().Bool(MCPAllowPasteFlag, false, "")
	cmd.Flags().Bool("public-only", true, "")
	cmd.SetContext(t.Context())
	if err := cmd.Flags().Set(MCPAllowPasteFlag, "true"); err != nil {
		t.Fatal(err)
	}
	if _, err := MCPTools(cmd, ""); err == nil {
		t.Fatal("paste grant accepted without a profile")
	}
	for key, value := range map[string]string{
		"phabricator.profiles.work.key":      "fixture-token",
		"phabricator.profiles.work.username": "example",
		"phabricator.profiles.work.url":      defaultPhabricatorURL,
	} {
		if err := mwconfig.PutKeyValueOnDisk(key, value); err != nil {
			t.Fatal(err)
		}
	}
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	http.DefaultTransport = authTransport(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/api/paste.search" {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL)
		}
		if err := req.ParseForm(); err != nil || !bytes.Contains([]byte(req.Form.Get("params")), []byte("fixture-token")) {
			t.Errorf("paste request did not use the selected profile: %v", err)
		}
		return pasteResponse(`{"result":{"data":[{"id":123,"phid":"PHID-PSTE-example","fields":{"title":"Example"},"attachments":{"content":{"content":"Fixture source"}}}]}}`), nil
	})
	if _, err := MCPTools(cmd, "work"); err == nil || !strings.Contains(err.Error(), "--public-only=false") {
		t.Fatalf("public-only server accepted paste read: %v", err)
	}
	if err := cmd.Flags().Set("public-only", "false"); err != nil {
		t.Fatal(err)
	}
	operations, err := MCPTools(cmd, "work")
	if err != nil || len(operations) != 6 {
		t.Fatalf("paste grant: %d tools, %v", len(operations), err)
	}
	server := mcpserver.New("test-server")
	for _, operation := range operations {
		operation.Register(server)
	}
	session := connectTestPhabricatorMCP(t, t.Context(), server)
	result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "phabricator_get_paste", Arguments: map[string]any{"ref": "P123"}})
	if err != nil || result.IsError {
		t.Fatalf("paste: %+v, %v", result, err)
	}
	raw, err := json.Marshal(result.StructuredContent)
	if err != nil || !bytes.Contains(raw, []byte("Fixture source")) {
		t.Fatalf("paste: %s, %v", raw, err)
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
		"phabricator.public_only":            "false",
		"access.public_only":                 "false",
	} {
		if err := mwconfig.PutKeyValueOnDisk(key, value); err != nil {
			t.Fatal(err)
		}
	}
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	searches := 0
	http.DefaultTransport = authTransport(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Errorf("unexpected profile request %s %s", req.Method, req.URL)
		}
		if err := req.ParseForm(); err != nil {
			return nil, err
		}
		var payload struct {
			Conduit     struct{ Token string } `json:"__conduit__"`
			Limit       int
			Order       string
			Constraints struct {
				Query       string   `json:"query"`
				Priorities  []int    `json:"priorities"`
				Assigned    []string `json:"assigned"`
				AuthorPHIDs []string
				Statuses    []string
				Projects    []string
				Usernames   []string
				Slugs       []string
			} `json:"constraints"`
		}
		if err := json.Unmarshal([]byte(req.FormValue("params")), &payload); err != nil {
			return nil, err
		}
		if payload.Conduit.Token != "fixture-token" {
			t.Error("request did not use the explicitly selected profile")
		}
		switch req.URL.Path {
		case "/api/user.whoami":
			return pasteResponse(`{"result":{"userName":"example"}}`), nil
		case "/api/user.search":
			if !reflect.DeepEqual(payload.Constraints.Usernames, []string{"example"}) {
				t.Errorf("unexpected author lookup: %+v", payload.Constraints)
			}
			return pasteResponse(`{"result":{"data":[{"phid":"PHID-USER-example","fields":{"username":"example"}}]}}`), nil
		case "/api/project.search":
			if reflect.DeepEqual(payload.Constraints.Slugs, []string{"ci"}) {
				return pasteResponse(`{"result":{"data":[{"id":1,"phid":"PHID-PROJ-ci","fields":{"name":"CI"}}]}}`), nil
			}
			if reflect.DeepEqual(payload.Constraints.Slugs, []string{"team"}) {
				return pasteResponse(`{"result":{"data":[{"id":2,"phid":"PHID-PROJ-team","fields":{"name":"Team"}}]}}`), nil
			}
			t.Errorf("unexpected project lookup: %+v", payload.Constraints)
			return pasteResponse(`{"result":{"data":[]}}`), nil
		case "/api/maniphest.search":
			searches++
		default:
			t.Errorf("unexpected request path %s", req.URL.Path)
		}
		if payload.Constraints.Query != "example" || !reflect.DeepEqual(payload.Constraints.Priorities, []int{80}) ||
			!reflect.DeepEqual(payload.Constraints.Assigned, []string{"none()"}) ||
			!reflect.DeepEqual(payload.Constraints.AuthorPHIDs, []string{"PHID-USER-example"}) ||
			!reflect.DeepEqual(payload.Constraints.Statuses, []string{"resolved"}) ||
			!reflect.DeepEqual(payload.Constraints.Projects, []string{"PHID-PROJ-ci", "PHID-PROJ-team"}) || payload.Limit != 50 || payload.Order != "newest" {
			t.Errorf("unexpected search payload: %+v", payload.Constraints)
		}
		return pasteResponse(`{"result":` + accessPage(t, accessTask(123, "public", wikimediaPublicSpace), accessTask(124, "users", wikimediaPublicSpace), accessTask(125, "", wikimediaPublicSpace)) + `}`), nil
	})
	var stdout bytes.Buffer
	cliCmd := NewPhabricatorCmd()
	cliCmd.SetOut(&stdout)
	cliCmd.SetArgs([]string{
		"task", "list", "--public-only", "--profile", "work", "--search", "example", "--priority", "80", "--unassigned", "--project", "ci,team",
		"--author", "me", "--status", "resolved", "--order", "newest", "--output", "json",
	})
	if err := cliCmd.Execute(); err != nil {
		t.Fatal(err)
	}
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
		Name: "phabricator_search_tasks",
		Arguments: map[string]any{
			"query": "example", "priorities": []int{80}, "unassigned": true,
			"projects": []string{"ci", "team"}, "author": "me", "statuses": []string{"resolved"}, "order": "newest",
		},
	})
	if err != nil || result.IsError {
		t.Fatalf("search: %+v, %v", result, err)
	}
	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var page taskPage
	if err := json.Unmarshal(raw, &page); err != nil || len(page.Data) != 2 || !page.Data[1].Redacted || page.Data[1].Fields.Name != nonPublicLabel ||
		page.Data[1].Fields.DateCreated != 0 || page.Data[1].Fields.DateModified != 0 {
		t.Fatalf("profile search did not apply the default public-only policy: %s, %v", raw, err)
	}
	var cliPage taskPage
	if err := json.Unmarshal(stdout.Bytes(), &cliPage); err != nil || len(cliPage.Data) != 2 || !reflect.DeepEqual(cliPage.Data[0], page.Data[0]) || searches != 2 {
		t.Fatalf("public task JSON or request count differs: CLI %s, MCP %s, searches %d, error %v", stdout.Bytes(), raw, searches, err)
	}
}
