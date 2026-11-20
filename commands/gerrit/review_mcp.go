package gerrit

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	gogerrit "github.com/andygrunwald/go-gerrit"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mcpserver"
)

type changeReadInput struct {
	ChangeID string `json:"change_id" jsonschema:"Numeric Wikimedia Gerrit change ID."`
}

func (input changeReadInput) path() (string, error) {
	id, err := strconv.ParseUint(input.ChangeID, 10, 64)
	if err != nil || id == 0 {
		return "", fmt.Errorf("provide a numeric Gerrit change ID")
	}
	return "/changes/" + strconv.FormatUint(id, 10), nil
}

type revisionReadInput struct {
	changeReadInput
	Revision string `json:"revision,omitempty" jsonschema:"Patch set number or full commit SHA. Reuse current_revision from get_change_details to pin the patch set. Defaults to current."`
}

func (input revisionReadInput) revisionPath() (string, error) {
	path, err := input.path()
	if err != nil {
		return "", err
	}
	switch input.Revision {
	case "", "current":
		return path + "/revisions/current", nil
	}
	if len(input.Revision) == 40 {
		if _, err := hex.DecodeString(input.Revision); err == nil {
			return path + "/revisions/" + input.Revision, nil
		}
	}
	if number, err := strconv.ParseUint(input.Revision, 10, 32); err == nil && number > 0 {
		return path + "/revisions/" + strconv.FormatUint(number, 10), nil
	}
	return "", fmt.Errorf("provide a Gerrit patch set number or full commit SHA")
}

func publicGerritJSON(ctx context.Context, path string) (json.RawMessage, error) {
	client, err := newGerritClientWithHTTP(ctx, http.Client{
		Timeout:       30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	})
	if err != nil {
		return nil, err
	}
	resp, err := client.Call(ctx, http.MethodGet, path, nil, nil)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Gerrit returned HTTP %d", resp.StatusCode)
	}
	const maxResponse = 8 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxResponse {
		return nil, fmt.Errorf("Gerrit response exceeds 8 MiB")
	}
	data = bytes.TrimSpace(gogerrit.RemoveMagicPrefixLine(data))
	if !json.Valid(data) {
		return nil, fmt.Errorf("Gerrit returned invalid JSON")
	}
	return data, nil
}

func gerritReadTool[In, Out any](name, description, command string, run mcpserver.Action[In, Out], fields ...string) mcpserver.Operation {
	return run.Tool(mcp.Tool{Name: name, Description: description, OutputSchema: map[string]string{"type": "object"}},
		mcpserver.ReadHints(commandmeta.DataPublic)).ForCommand(command).WithOutbound("gerrit.wikimedia.org", fields...)
}

func changeReadTool(name, description, command, suffix, resultKey string) mcpserver.Operation {
	return gerritReadTool(name, description, command, func(ctx context.Context, input changeReadInput) (map[string]json.RawMessage, error) {
		path, err := input.path()
		if err != nil {
			return nil, err
		}
		data, err := publicGerritJSON(ctx, path+suffix)
		if err != nil {
			return nil, err
		}
		return map[string]json.RawMessage{resultKey: data}, nil
	}, "change ID")
}

// MCPTools lists the anonymous Gerrit review tools.
func MCPTools() []mcpserver.Operation {
	return []mcpserver.Operation{
		gerritReadTool("query_changes", "Search public Wikimedia Gerrit changes. Gerrit receives the query.", "gerrit changes list", queryChanges, "search query", "result limit"),
		gerritReadTool("get_change_details", "Read a public change with labels, patch set, and full review and CI messages.", "gerrit changes detail", changeDetails, "change ID"),
		gerritReadTool("get_commit_message", "Read the verbatim commit message for the selected patch set.", "gerrit changes message get", commitMessage, "change ID", "patch set"),
		gerritReadTool("list_change_files", "List files in the selected patch set.", "gerrit changes revisions files list", listChangeFiles, "change ID", "patch set"),
		gerritReadTool("get_file_diff", "Read Gerrit's structured diff for a file in the selected patch set.", "gerrit changes revisions files diff", fileDiff, "change ID", "patch set", "file path"),
		gerritReadTool("list_change_comments", "Read published comments grouped by file, with reply IDs. Set revision to limit comments to one patch set.", "gerrit changes comments", listChangeComments, "change ID", "patch set"),
		changeReadTool("changes_submitted_together", "Read changes Gerrit would submit together with this change.", "gerrit changes submitted-together", "/submitted_together/", "changes"),
	}
}

type changeQueryInput struct {
	Query string `json:"query" jsonschema:"Gerrit search query. Use public terms."`
	Limit int    `json:"limit,omitempty" jsonschema:"Maximum results, 1 through 100. Defaults to 25."`
}

func queryChanges(ctx context.Context, input changeQueryInput) (map[string]json.RawMessage, error) {
	if strings.TrimSpace(input.Query) == "" || input.Limit < 0 || input.Limit > 100 {
		return nil, fmt.Errorf("provide a query and use a limit between 1 and 100")
	}
	if input.Limit == 0 {
		input.Limit = 25
	}
	params := url.Values{"q": {input.Query}, "n": {strconv.Itoa(input.Limit)}}
	data, err := publicGerritJSON(ctx, "/changes/?"+params.Encode())
	return map[string]json.RawMessage{"changes": data}, err
}

type changeDetailsInput struct {
	changeReadInput
	FullMessages bool `json:"full_messages,omitempty" jsonschema:"Request full messages. This tool always returns them."`
}

func changeDetails(ctx context.Context, input changeDetailsInput) (json.RawMessage, error) {
	path, err := input.path()
	if err != nil {
		return nil, err
	}
	return publicGerritJSON(ctx, path+"/detail/?o=CURRENT_REVISION&o=CURRENT_COMMIT&o=DETAILED_LABELS&o=MESSAGES")
}

func listChangeFiles(ctx context.Context, input revisionReadInput) (map[string]json.RawMessage, error) {
	path, err := input.revisionPath()
	if err != nil {
		return nil, err
	}
	data, err := publicGerritJSON(ctx, path+"/files/")
	return map[string]json.RawMessage{"files": data}, err
}

func listChangeComments(ctx context.Context, input revisionReadInput) (map[string]json.RawMessage, error) {
	var path string
	var err error
	if input.Revision == "" {
		path, err = input.path()
	} else {
		path, err = input.revisionPath()
	}
	if err != nil {
		return nil, err
	}
	data, err := publicGerritJSON(ctx, path+"/comments/")
	return map[string]json.RawMessage{"comments": data}, err
}

func commitMessage(ctx context.Context, input revisionReadInput) (map[string]string, error) {
	path, err := input.revisionPath()
	if err != nil {
		return nil, err
	}
	data, err := publicGerritJSON(ctx, path+"/commit/")
	if err != nil {
		return nil, err
	}
	var response struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(data, &response); err != nil || response.Message == "" {
		return nil, fmt.Errorf("Gerrit did not return a commit message")
	}
	return map[string]string{"change_id": input.ChangeID, "verbatim_commit_message": response.Message}, nil
}

type fileDiffInput struct {
	revisionReadInput
	FilePath string `json:"file_path" jsonschema:"Exact path as listed by Gerrit, including /COMMIT_MSG or /MERGE_LIST."`
}

func fileDiff(ctx context.Context, input fileDiffInput) (json.RawMessage, error) {
	path, err := input.revisionPath()
	if err != nil {
		return nil, err
	}
	file := input.FilePath
	if file == "/COMMIT_MSG" || file == "/MERGE_LIST" {
		file = strings.TrimPrefix(file, "/")
	}
	for _, part := range strings.Split(file, "/") {
		if part == "" || part == "." || part == ".." || strings.ContainsAny(part, "\\\x00") {
			return nil, fmt.Errorf("provide a relative Gerrit file path")
		}
	}
	return publicGerritJSON(ctx, path+"/files/"+url.PathEscape(input.FilePath)+"/diff/")
}
