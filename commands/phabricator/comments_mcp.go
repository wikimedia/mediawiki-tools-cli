package phabricator

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mcpserver"
)

func readCommentsTool(client anonymousPhabricator, name string) mcpserver.Operation {
	type result struct {
		TaskID   string             `json:"taskId"`
		Comments []anonymousComment `json:"comments"`
	}
	run := mcpserver.Action[taskReadInput, result](func(ctx context.Context, input taskReadInput) (result, error) {
		ref, err := input.reference()
		if err != nil {
			return result{}, err
		}
		task, comments, err := client.task(ctx, ref)
		if err != nil {
			return result{}, err
		}
		return result{TaskID: taskReference(*task), Comments: comments}, nil
	})
	return run.Tool(mcp.Tool{
		Name: name, Description: "Read public comments on a Wikimedia Phabricator task.",
	}, mcpserver.ReadHints(commandmeta.DataPublic)).ForCommand("phabricator comments")
}
