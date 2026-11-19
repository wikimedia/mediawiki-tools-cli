package phabricator

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mcpserver"
)

type objectReadInput struct {
	Ref string `json:"ref" jsonschema:"A Phabricator object ID such as T123 or P123, or a URL on the selected Phabricator site."`
}

func readTaskTool(client anonymousPhabricator) mcpserver.Operation {
	run := mcpserver.Action[objectReadInput, *phabTask](func(ctx context.Context, input objectReadInput) (*phabTask, error) {
		task, _, err := client.task(ctx, input.Ref)
		return task, err
	})
	return run.Tool(mcp.Tool{
		Name: "phabricator_get_task", Description: "Read a public Wikimedia Phabricator task and its description.",
		OutputSchema: map[string]any{"type": "object"},
	}, mcpserver.ReadHints(commandmeta.DataPublic)).ForCommand("phabricator task view")
}

func readPasteTool(client *conduitClient) mcpserver.Operation {
	run := mcpserver.Action[objectReadInput, *phabPaste](func(ctx context.Context, input objectReadInput) (*phabPaste, error) {
		if client == nil {
			return nil, fmt.Errorf("paste reads need an explicitly authorized Phabricator profile")
		}
		return client.getPaste(ctx, input.Ref)
	})
	return run.Tool(mcp.Tool{
		Name: "phabricator_get_paste", Description: "Read a paste and its source using the authorized Phabricator account. Results can include private content.",
	}, mcpserver.ReadHints(commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata)).
		ForCommand("phabricator paste view").WithRequirement("Phabricator profile and --" + MCPAllowPasteFlag)
}
