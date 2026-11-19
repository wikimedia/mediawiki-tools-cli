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

func NewOperation[In, Out any](name, description string, hints commandmeta.Hints, handler mcp.ToolHandlerFor[In, Out]) Operation {
	return Operation{
		name: name, description: description, hints: hints,
		register: func(server *mcp.Server) {
			mcp.AddTool(server, &mcp.Tool{
				Name: name, Description: description,
				Annotations: &mcp.ToolAnnotations{
					ReadOnlyHint: hints.ReadOnlyHint, DestructiveHint: &hints.DestructiveHint,
					IdempotentHint: hints.IdempotentHint, OpenWorldHint: &hints.OpenWorldHint,
				},
				Meta: mcp.Meta{
					"a-wikimedia-cli/dataAccess": hints.DataAccess,
					"a-wikimedia-cli/hints":      hints,
				},
			}, handler)
		},
	}
}

func (o Operation) Register(server *mcp.Server) {
	o.register(server)
}
