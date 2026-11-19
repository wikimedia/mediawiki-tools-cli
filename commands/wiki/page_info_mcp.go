package wiki

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mcpserver"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiapi"
)

func pageInfoTool() mcpserver.Operation {
	run := readPageInfo(func(input pageInfoInput) (*wikiapi.Client, error) { return publicWikiMCPClient(input.Wiki) })
	return run.Tool(mcp.Tool{
		Name: "wiki_page_info", Description: "Read native page identity, URLs and protection metadata on MediaWiki.org or English Wikipedia. The selected public wiki receives the page title.",
		OutputSchema: map[string]any{"type": "object"},
	}, mcpserver.ReadHints(commandmeta.DataPublic)).ForCommand("wiki page info")
}
