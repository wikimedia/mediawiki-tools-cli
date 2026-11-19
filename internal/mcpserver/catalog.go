package mcpserver

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
)

// Service contains only operations authorized by the process launcher.
type Service struct {
	Name     string
	Tools    []Operation
	Deferred bool
}

type catalog struct {
	server   *mcp.Server
	services []Service
	mu       sync.Mutex
}

type discoverInput struct {
	Query   string `json:"query,omitempty" jsonschema:"Words to match in the service, tool name or description."`
	Service string `json:"service,omitempty" jsonschema:"Only show operations for this service."`
	Offset  int    `json:"offset,omitempty" jsonschema:"Offset returned as nextOffset by the previous page."`
	Limit   int    `json:"limit,omitempty" jsonschema:"Number of operations to return. Defaults to 20, maximum 50."`
}

type operationInfo struct {
	Service     string            `json:"service"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Enabled     bool              `json:"enabled"`
	Hints       commandmeta.Hints `json:"hints"`
}

type discoverResult struct {
	Tools      []operationInfo `json:"tools"`
	NextOffset int             `json:"nextOffset,omitempty"`
}

type enableInput struct {
	Service string `json:"service" jsonschema:"Service already authorized by the launcher. Discover its tool group first."`
}

type enableResult struct {
	Service string   `json:"service"`
	Tools   []string `json:"tools"`
}

func RegisterCatalog(server *mcp.Server, services []Service) {
	c := &catalog{server: server, services: services}
	hasDeferred := false
	for _, service := range services {
		if service.Deferred {
			hasDeferred = true
			continue
		}
		for _, tool := range service.Tools {
			tool.Register(server)
		}
	}

	discoverHints := ReadHints(commandmeta.DataNone)
	discoverHints.OpenWorldHint = false
	NewOperation("mw_discover_tools", "Find Wikimedia CLI operations authorized for this server and see which services can be enabled.",
		discoverHints, c.discover).Register(server)
	if hasDeferred {
		enableHints := commandmeta.Hints{
			ToolAnnotations: commandmeta.ToolAnnotations{IdempotentHint: true},
			DataAccess:      []commandmeta.DataAccess{commandmeta.DataNone},
		}
		NewOperation("mw_enable_service", "Enable a service authorized at launch. This updates the tool catalog for this local MCP process. Refresh the tool list after enabling.",
			enableHints, c.enable).Register(server)
	}
}

func (c *catalog) discover(_ context.Context, _ *mcp.CallToolRequest, input discoverInput) (*mcp.CallToolResult, discoverResult, error) {
	limit := input.Limit
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > 50 || input.Offset < 0 {
		return nil, discoverResult{}, fmt.Errorf("use a limit between 1 and 50 and a nonnegative offset")
	}
	words := strings.Fields(strings.ToLower(input.Query))
	result := discoverResult{Tools: make([]operationInfo, 0, limit)}
	matched := 0
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, service := range c.services {
		if input.Service != "" && input.Service != service.Name {
			continue
		}
		for _, tool := range service.Tools {
			if !matchesOperation(words, service.Name+" "+tool.name+" "+tool.description) {
				continue
			}
			matched++
			if matched <= input.Offset {
				continue
			}
			if len(result.Tools) == limit {
				result.NextOffset = matched - 1
				return nil, result, nil
			}
			result.Tools = append(result.Tools, operationInfo{
				Service: service.Name, Name: tool.name, Description: tool.description,
				Enabled: !service.Deferred, Hints: tool.hints,
			})
		}
	}
	return nil, result, nil
}

func matchesOperation(words []string, text string) bool {
	text = strings.ToLower(text)
	for _, word := range words {
		if !strings.Contains(text, word) {
			return false
		}
	}
	return true
}

func (c *catalog) enable(_ context.Context, _ *mcp.CallToolRequest, input enableInput) (*mcp.CallToolResult, enableResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.services {
		service := &c.services[i]
		if service.Name != input.Service {
			continue
		}
		result := enableResult{Service: service.Name, Tools: make([]string, 0, len(service.Tools))}
		for _, tool := range service.Tools {
			if service.Deferred {
				tool.Register(c.server)
			}
			result.Tools = append(result.Tools, tool.name)
		}
		service.Deferred = false
		return nil, result, nil
	}
	return nil, enableResult{}, fmt.Errorf("service %q was not authorized at launch", input.Service)
}
