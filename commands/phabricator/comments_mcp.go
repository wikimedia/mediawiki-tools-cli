package phabricator

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mcpserver"
)

func readCommentsTool(client anonymousPhabricator) mcpserver.Operation {
	type result struct {
		Comments []anonymousComment `json:"comments"`
	}
	run := mcpserver.Action[objectReadInput, result](func(ctx context.Context, input objectReadInput) (result, error) {
		_, comments, err := client.task(ctx, input.Ref)
		return result{Comments: comments}, err
	})
	return run.Tool(mcp.Tool{
		Name: "phabricator_get_comments", Description: "Read public comments on a Wikimedia Phabricator task.",
	}, mcpserver.ReadHints(commandmeta.DataPublic)).ForCommand("phabricator comments")
}
