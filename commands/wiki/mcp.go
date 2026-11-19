package wiki

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mcpserver"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiapi"
)

type mcpPageInfoInput struct {
	Title string `json:"title" jsonschema:"Title of one wiki page, for example API:Main page."`
	Wiki  string `json:"wiki,omitempty" jsonschema:"mediawiki (default) or enwiki (English Wikipedia)."`
}

type mcpPageInfoResult struct {
	Title          string `json:"title"`
	URI            string `json:"uri"`
	PageID         int64  `json:"pageId"`
	LastRevisionID int64  `json:"lastRevisionId"`
}

// MCPTools exposes anonymous reads from a fixed set of public wikis.
func MCPTools() []mcpserver.Operation {
	return []mcpserver.Operation{mcpserver.NewOperation("wiki_page_info",
		"Read a public page's canonical title, URL, page ID and latest revision ID on MediaWiki.org or English Wikipedia.",
		mcpserver.ReadHints(commandmeta.DataPublic),
		func(ctx context.Context, _ *mcp.CallToolRequest, input mcpPageInfoInput) (*mcp.CallToolResult, mcpPageInfoResult, error) {
			var result mcpPageInfoResult
			var endpoint string
			switch input.Wiki {
			case "", "mediawiki":
				endpoint = "https://www.mediawiki.org/w/api.php"
			case "enwiki":
				endpoint = "https://en.wikipedia.org/w/api.php"
			default:
				return nil, result, fmt.Errorf("choose mediawiki or enwiki")
			}
			client, err := wikiapi.New(endpoint, &http.Client{Timeout: 30 * time.Second})
			if err != nil {
				return nil, result, err
			}
			page, err := client.PageInfo(ctx, strings.TrimSpace(input.Title), true)
			if err != nil {
				return nil, result, err
			}
			if err = json.Unmarshal(page["title"], &result.Title); err != nil {
				return nil, result, err
			}
			if err = json.Unmarshal(page["fullurl"], &result.URI); err != nil {
				return nil, result, err
			}
			if err = json.Unmarshal(page["pageid"], &result.PageID); err != nil {
				return nil, result, err
			}
			if err = json.Unmarshal(page["lastrevid"], &result.LastRevisionID); err != nil {
				return nil, result, err
			}
			return nil, result, nil
		})}
}
