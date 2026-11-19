package wiki

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mcpserver"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiapi"
)

type pageSourceInput struct {
	Title string `json:"title,omitempty" jsonschema:"Page title sent to the selected public wiki. Defaults to its main page."`
	Wiki  string `json:"wiki,omitempty" jsonschema:"mediawiki (default) or enwiki (English Wikipedia)."`
}

func pageSourceTool() mcpserver.Operation {
	run := mcpserver.Action[pageSourceInput, wikiapi.PageSource](func(ctx context.Context, input pageSourceInput) (wikiapi.PageSource, error) {
		client, err := publicWikiMCPClient(input.Wiki)
		if err != nil {
			return wikiapi.PageSource{}, err
		}
		return client.PageSource(ctx, input.Title)
	})
	return run.Tool(mcp.Tool{
		Name: "wiki_get_page", Description: "Read a public wiki page’s source on MediaWiki.org or English Wikipedia.",
	}, mcpserver.ReadHints(commandmeta.DataPublic)).ForCommand("wiki page get")
}
