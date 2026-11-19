package wiki

import (
	"fmt"
	"net/http"
	"time"

	"gitlab.wikimedia.org/repos/releng/cli/internal/mcpserver"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiapi"
)

// MCPTools lists the wiki commands available through the MCP service.
func MCPTools() []mcpserver.Operation {
	return []mcpserver.Operation{pageInfoTool(), pageSourceTool()}
}

func publicWikiMCPClient(wiki string) (*wikiapi.Client, error) {
	var endpoint string
	switch wiki {
	case "", "mediawiki":
		endpoint = "https://www.mediawiki.org/w/api.php"
	case "enwiki":
		endpoint = "https://en.wikipedia.org/w/api.php"
	default:
		return nil, fmt.Errorf("choose mediawiki or enwiki")
	}
	return wikiapi.New(endpoint, &http.Client{Timeout: 30 * time.Second})
}
