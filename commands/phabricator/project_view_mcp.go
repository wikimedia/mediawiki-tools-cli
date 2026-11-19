package phabricator

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mcpserver"
)

func projectViewTool(client anonymousPhabricator) mcpserver.Operation {
	type input struct {
		Identifier string `json:"identifier" jsonschema:"Project tag, exact name, or numeric ID."`
	}
	run := mcpserver.Action[input, *phabProject](func(ctx context.Context, request input) (*phabProject, error) {
		return client.project(ctx, request.Identifier)
	})
	return run.Tool(mcp.Tool{
		Name: "phabricator_get_project", Description: "Look up a public Wikimedia Phabricator project.",
	}, mcpserver.ReadHints(commandmeta.DataPublic)).ForCommand("phabricator project view")
}
