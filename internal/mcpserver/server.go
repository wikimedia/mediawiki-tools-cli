package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
)

func New(name string) *mcp.Server {
	return mcp.NewServer(&mcp.Implementation{Name: name, Version: cli.UserAgentVersion()}, nil)
}

func Serve(ctx context.Context, server *mcp.Server) error {
	return server.Run(ctx, &mcp.StdioTransport{})
}

func ReadHints(access ...commandmeta.DataAccess) commandmeta.Hints {
	return commandmeta.Hints{
		ToolAnnotations: commandmeta.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: true},
		DataAccess:      access,
	}
}

// Operation keeps a tool's discoverable metadata together with its typed handler.
type Operation struct {
	name        string
	description string
	hints       commandmeta.Hints
	register    func(*mcp.Server)
}

// Action is an operation shared by a CLI command and an MCP tool. Define it
// beside the CLI command and pass the same result to CLI JSON output and Tool.
// Each entry point binds its permitted backend before invoking the action.
type Action[In, Out any] func(context.Context, In) (Out, error)

func (action Action[In, Out]) Tool(tool mcp.Tool, hints commandmeta.Hints) Operation {
	return newOperation(tool, hints, func(ctx context.Context, _ *mcp.CallToolRequest, input In) (*mcp.CallToolResult, Out, error) {
		result, err := action(ctx, input)
		return nil, result, err
	})
}

func NewOperation[In, Out any](name, description string, hints commandmeta.Hints, handler mcp.ToolHandlerFor[In, Out]) Operation {
	return newOperation(mcp.Tool{Name: name, Description: description}, hints, handler)
}

func newOperation[In, Out any](tool mcp.Tool, hints commandmeta.Hints, handler mcp.ToolHandlerFor[In, Out]) Operation {
	tool.Annotations = &mcp.ToolAnnotations{
		ReadOnlyHint: hints.ReadOnlyHint, DestructiveHint: &hints.DestructiveHint,
		IdempotentHint: hints.IdempotentHint, OpenWorldHint: &hints.OpenWorldHint,
	}
	tool.Meta = mcp.Meta{
		"a-wikimedia-cli/dataAccess": hints.DataAccess,
		"a-wikimedia-cli/hints":      hints,
	}
	return Operation{
		name: tool.Name, description: tool.Description, hints: hints,
		register: func(server *mcp.Server) {
			definition := tool
			mcp.AddTool(server, &definition, handler)
		},
	}
}

func (o Operation) Register(server *mcp.Server) {
	o.register(server)
}
