package mcpserver

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
)

func TestCatalogDiscoveryAndActivation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server := New("catalog-test")
	fixture := func(name, description string) Operation {
		return NewOperation(name, description, ReadHints(commandmeta.DataPublic),
			func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, struct{}, error) {
				return nil, struct{}{}, nil
			})
	}
	RegisterCatalog(server, []Service{
		{Name: "phab", Tools: []Operation{fixture("phabricator_search_tasks", "Search tasks")}},
		{Name: "wiki", Deferred: true, Tools: []Operation{fixture("wiki_page_info", "Read wiki page metadata")}},
	})
	changed := make(chan struct{}, 1)
	host := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, &mcp.ClientOptions{
		ToolListChangedHandler: func(context.Context, *mcp.ToolListChangedRequest) { changed <- struct{}{} },
	})
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	session, err := host.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	list := func() []string {
		t.Helper()
		result, err := session.ListTools(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, tool := range result.Tools {
			names = append(names, tool.Name)
			hints, ok := tool.Meta["a-wikimedia-cli/hints"].(map[string]any)
			if tool.Annotations == nil || !ok || hints["secretInput"] != false ||
				hints["highRisk"] != false || hints["readOnlyHint"] != tool.Annotations.ReadOnlyHint {
				t.Fatalf("tool lacks capability hints: %+v", tool)
			}
			if tool.Name == "mw_enable_service" && (tool.Annotations.ReadOnlyHint ||
				tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint ||
				tool.Annotations.OpenWorldHint == nil || *tool.Annotations.OpenWorldHint) {
				t.Fatalf("activation has incorrect hints: %+v", tool.Annotations)
			}
		}
		return names
	}
	call := func(name string, args any, wantError bool) *mcp.CallToolResult {
		t.Helper()
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil || result.IsError != wantError {
			t.Fatalf("%s: result=%+v, error=%v", name, result, err)
		}
		return result
	}
	discover := func(input discoverInput) discoverResult {
		t.Helper()
		result := call("mw_discover_tools", input, false)
		raw, err := json.Marshal(result.StructuredContent)
		if err != nil {
			t.Fatal(err)
		}
		var page discoverResult
		if err := json.Unmarshal(raw, &page); err != nil {
			t.Fatal(err)
		}
		return page
	}

	if names := list(); len(names) != 3 || slices.Contains(names, "wiki_page_info") {
		t.Fatalf("unexpected initial tools: %v", names)
	}
	first := discover(discoverInput{Limit: 1})
	if len(first.Tools) != 1 || !first.Tools[0].Enabled || first.NextOffset != 1 {
		t.Fatalf("unexpected first page: %+v", first)
	}
	page := discover(discoverInput{Offset: first.NextOffset})
	if len(page.Tools) != 1 || page.Tools[0].Enabled || page.Tools[0].Service != "wiki" ||
		!page.Tools[0].Hints.ReadOnlyHint || !slices.Equal(page.Tools[0].Hints.DataAccess, []commandmeta.DataAccess{commandmeta.DataPublic}) {
		t.Fatalf("unexpected deferred operation: %+v", page)
	}

	call("mw_enable_service", enableInput{Service: "unapproved"}, true)
	if names := list(); len(names) != 3 {
		t.Fatalf("unapproved activation changed tools: %v", names)
	}
	call("mw_enable_service", enableInput{Service: "wiki"}, false)
	select {
	case <-changed:
	case <-ctx.Done():
		t.Fatal("no tool list change notification")
	}
	if names := list(); len(names) != 4 || !slices.Contains(names, "wiki_page_info") {
		t.Fatalf("unexpected refreshed tools: %v", names)
	}
	call("mw_enable_service", enableInput{Service: "wiki"}, false)
	call("wiki_page_info", struct{}{}, false)
	page = discover(discoverInput{Query: "page metadata"})
	if len(page.Tools) != 1 || !page.Tools[0].Enabled {
		t.Fatalf("unexpected active operation: %+v", page)
	}
}
