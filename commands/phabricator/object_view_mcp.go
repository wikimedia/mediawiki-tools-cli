package phabricator

import (
	"context"
	"fmt"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mcpserver"
)

type objectReadInput struct {
	Ref string `json:"ref" jsonschema:"A Phabricator object ID such as T123 or P123, or a URL on the selected Phabricator site."`
}

type taskReadInput struct {
	Ref    string `json:"ref,omitempty" jsonschema:"Task reference or public Wikimedia Phabricator task URL."`
	TaskID string `json:"taskId,omitempty" jsonschema:"Task ID, such as T123 or 123. Supply ref or taskId."`
}

func (input taskReadInput) reference() (string, error) {
	if input.Ref != "" && input.TaskID != "" {
		return "", fmt.Errorf("supply ref or taskId")
	}
	if input.Ref != "" {
		return input.Ref, nil
	}
	if input.TaskID == "" {
		return "", fmt.Errorf("supply ref or taskId")
	}
	if id, err := strconv.Atoi(input.TaskID); err == nil && id > 0 {
		return "T" + strconv.Itoa(id), nil
	}
	return input.TaskID, nil
}

func readTaskTool(client anonymousPhabricator) mcpserver.Operation {
	type taskDetails phabTask
	type result struct {
		*taskDetails
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
		return result{taskDetails: (*taskDetails)(task), Comments: comments}, nil
	})
	return run.Tool(mcp.Tool{
		Name: "phabricator_get_task", Description: "Read a public Wikimedia Phabricator task, its description and public comments.",
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
