package mcpserver

import (
	"context"
	"fmt"
	"strings"

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
	register    func(*mcp.Server, ToolAccess, string)
	command     string
	requirement string
	outbound    *Outbound
}

// Outbound describes where the tool sends request data and what it sends.
type Outbound struct {
	Destination string   `json:"destination"`
	Fields      []string `json:"fields"`
}

type ToolAccess struct {
	Hints    commandmeta.Hints `json:"hints"`
	Outbound *Outbound         `json:"outbound,omitempty"`
}

// CommandTool describes a tool's relationship to a CLI command for the manual.
type CommandTool struct {
	Command     string `json:"-"`
	Name        string `json:"name"`
	Requirement string `json:"requirement,omitempty"`
	ToolAccess
	Profile *ToolAccess `json:"profile,omitempty"`
}

// ForCommand records the canonical command path relative to the CLI root.
func (o Operation) ForCommand(path string) Operation {
	o.command = path
	return o
}

func (o Operation) WithRequirement(requirement string) Operation {
	o.requirement = requirement
	return o
}

func (o Operation) WithOutbound(destination string, fields ...string) Operation {
	o.outbound = &Outbound{Destination: destination, Fields: fields}
	return o
}

func (o Operation) access() ToolAccess {
	return ToolAccess{Hints: o.hints, Outbound: o.outbound}
}

func (o Operation) CommandTool() CommandTool {
	return CommandTool{Command: o.command, Name: o.name, Requirement: o.requirement, ToolAccess: o.access()}
}

// Action is an operation shared by a CLI command and an MCP tool. Define it
// beside the CLI command and pass the same result to CLI JSON output and Tool.
// Put the tool adapter in a companion _mcp.go file.
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
	return Operation{
		name: tool.Name, description: tool.Description, hints: hints,
		register: func(server *mcp.Server, access ToolAccess, requirement string) {
			definition := tool
			definition.Meta = mcp.Meta{
				"a-wikimedia-cli/dataAccess": access.Hints.DataAccess,
				"a-wikimedia-cli/hints":      access.Hints,
			}
			if access.Outbound != nil {
				definition.Meta["a-wikimedia-cli/outbound"] = access.Outbound
			}
			if requirement != "" {
				definition.Meta["a-wikimedia-cli/requirement"] = requirement
			}
			mcp.AddTool(server, &definition, handler)
		},
	}
}

func (o Operation) Register(server *mcp.Server) {
	o.register(server, o.access(), o.requirement)
}

// ValidateOperations checks a server's full catalog before any tool is registered.
func ValidateOperations(operations []Operation) error {
	seen := make(map[string]bool)
	for _, operation := range operations {
		if seen[operation.name] {
			return fmt.Errorf("duplicate MCP tool name %q", operation.name)
		}
		seen[operation.name] = true
		if err := operation.access().Validate(); err != nil {
			return fmt.Errorf("MCP tool %q: %w", operation.name, err)
		}
	}
	return nil
}

func (access ToolAccess) Validate() error {
	if err := access.Hints.Validate(); err != nil {
		return err
	}
	if access.Hints.OpenWorldHint && access.Outbound == nil {
		return fmt.Errorf("external tools require outbound metadata")
	}
	if access.Outbound != nil {
		if strings.TrimSpace(access.Outbound.Destination) == "" || len(access.Outbound.Fields) == 0 {
			return fmt.Errorf("outbound metadata requires a destination and request fields")
		}
		for _, field := range access.Outbound.Fields {
			if strings.TrimSpace(field) == "" {
				return fmt.Errorf("outbound request fields cannot be empty")
			}
		}
	}
	return nil
}
