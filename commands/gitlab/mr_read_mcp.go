package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mcpserver"
)

const publicGitLabHost = "gitlab.wikimedia.org"

type mrReadInput struct {
	Project string `json:"project,omitempty" jsonschema:"Public GitLab project path (namespace/project) or numeric project ID. Supply with mr_iid."`
	MRIID   int64  `json:"mr_iid,omitempty" jsonschema:"Merge request number within the project. Supply with project."`
	MRURL   string `json:"mr_url,omitempty" jsonschema:"Full Wikimedia GitLab merge request URL. Use on its own."`
}

func (input mrReadInput) target() (string, int64, error) {
	project, iid := input.Project, input.MRIID
	if input.MRURL != "" {
		if project != "" || iid != 0 {
			return "", 0, fmt.Errorf("provide either mr_url or project and mr_iid")
		}
		u, err := url.Parse(input.MRURL)
		if err != nil || u.Scheme != "https" || u.Host != publicGitLabHost || u.User != nil {
			return "", 0, fmt.Errorf("provide an HTTPS merge request URL on %s", publicGitLabHost)
		}
		projectPath, number, found := strings.Cut(strings.TrimPrefix(u.Path, "/"), "/-/merge_requests/")
		iid, err = strconv.ParseInt(strings.TrimSuffix(number, "/"), 10, 64)
		if !found || err != nil {
			return "", 0, fmt.Errorf("provide a GitLab merge request URL")
		}
		project = projectPath
	}
	if project == "" || iid < 1 {
		return "", 0, fmt.Errorf("provide a project and positive merge request number")
	}
	for _, part := range strings.Split(project, "/") {
		if part == "" || part == "." || part == ".." {
			return "", 0, fmt.Errorf("provide a valid GitLab project path or ID")
		}
	}
	return project, iid, nil
}

type publicMergeRequests struct {
	http *http.Client
	api  *gitlab.Client
}

func newPublicMergeRequests() (*publicMergeRequests, error) {
	httpClient := cli.WithHTTPUserAgent(&http.Client{
		Timeout:       30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	})
	api, err := gitlab.NewClient("", gitlab.WithBaseURL("https://"+publicGitLabHost+"/api/v4/"),
		gitlab.WithHTTPClient(httpClient), gitlab.WithCustomRetryMax(0), gitlab.WithUserAgent(cli.HTTPUserAgent()))
	if err != nil {
		return nil, err
	}
	return &publicMergeRequests{http: httpClient, api: api}, nil
}

func (client *publicMergeRequests) projectID(ctx context.Context, project string) (int64, error) {
	if id, err := strconv.ParseInt(project, 10, 64); err == nil && id > 0 {
		return id, nil
	}
	// Numeric REST paths also work under the sandbox's URL path restrictions.
	const query = `query($fullPath: ID!) { project(fullPath: $fullPath) { id } }`
	var data struct {
		Project struct{ ID string }
	}
	if err := gitLabGraphQL(ctx, client.http, query, map[string]string{"fullPath": project}, &data); err != nil {
		return 0, err
	}
	number, found := strings.CutPrefix(data.Project.ID, "gid://gitlab/Project/")
	id, err := strconv.ParseInt(number, 10, 64)
	if !found || err != nil || id < 1 {
		return 0, fmt.Errorf("public GitLab project %q not found", project)
	}
	return id, nil
}

// Only fixed read queries are sent. Caller input is passed in the variables.
func gitLabGraphQL(ctx context.Context, client *http.Client, query string, variables map[string]string, data any) error {
	encoded, err := json.Marshal(variables)
	if err != nil {
		return err
	}
	params := url.Values{"query": {query}, "variables": {string(encoded)}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+publicGitLabHost+"/api/graphql?"+params.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("GitLab returned HTTP %d", response.StatusCode)
	}
	envelope := struct {
		Data   any
		Errors []struct{ Message string }
	}{Data: data}
	if err := json.NewDecoder(io.LimitReader(response.Body, 8<<20)).Decode(&envelope); err != nil {
		return err
	}
	if len(envelope.Errors) > 0 {
		return fmt.Errorf("GitLab query: %s", envelope.Errors[0].Message)
	}
	return nil
}

// MCPTools lists the anonymous GitLab merge request reads.
func MCPTools() []mcpserver.Operation {
	return []mcpserver.Operation{
		gitLabReadTool("gitlab_get_merge_request", "Read a public Wikimedia GitLab merge request by URL or project and number.", "gitlab mr view", readMergeRequest),
		gitLabReadTool("gitlab_get_diff", "Read a page of up to 100 file diffs for a public Wikimedia GitLab merge request. Pass next_page as page to continue, even if a file filter found no matches.", "gitlab mr diff", readMergeRequestDiff, "page"),
		gitLabReadTool("gitlab_list_merge_request_comments", "Read a page of up to 100 public GitLab merge request discussion threads with inline comments. Pass next_cursor as after to continue.", "gitlab mr view", readMergeRequestComments, "discussion cursor"),
	}
}

func gitLabReadTool[In, Out any](name, description, command string, run mcpserver.Action[In, Out], fields ...string) mcpserver.Operation {
	return run.Tool(mcp.Tool{Name: name, Description: description, OutputSchema: map[string]string{"type": "object"}},
		mcpserver.ReadHints(commandmeta.DataPublic)).ForCommand(command).WithOutbound(publicGitLabHost, append([]string{"project identifier", "merge request number"}, fields...)...)
}

func readMergeRequest(ctx context.Context, input mrReadInput) (*gitlab.MergeRequest, error) {
	project, iid, err := input.target()
	if err != nil {
		return nil, err
	}
	client, err := newPublicMergeRequests()
	if err != nil {
		return nil, err
	}
	id, err := client.projectID(ctx, project)
	if err != nil {
		return nil, err
	}
	result, _, err := client.api.MergeRequests.GetMergeRequest(id, iid, nil, gitlab.WithContext(ctx))
	return result, err
}

type mrDiffInput struct {
	mrReadInput
	FilePath string `json:"file_path,omitempty" jsonschema:"Return matches for this old or new file path on the requested page."`
	Page     int64  `json:"page,omitempty" jsonschema:"Page to read. Starts at 1. Use next_page from the previous response."`
}

type mergeRequestDiffResult struct {
	Project   string                     `json:"project"`
	MRIID     int64                      `json:"mr_iid"`
	FileCount int                        `json:"file_count"`
	Truncated bool                       `json:"truncated"`
	NextPage  int64                      `json:"next_page,omitempty"`
	Diffs     []*gitlab.MergeRequestDiff `json:"diffs"`
}

func readMergeRequestDiff(ctx context.Context, input mrDiffInput) (mergeRequestDiffResult, error) {
	var result mergeRequestDiffResult
	if input.Page < 0 {
		return result, fmt.Errorf("page cannot be negative")
	}
	project, iid, err := input.target()
	if err != nil {
		return result, err
	}
	client, err := newPublicMergeRequests()
	if err != nil {
		return result, err
	}
	id, err := client.projectID(ctx, project)
	if err != nil {
		return result, err
	}
	const pageSize = 100
	page := max(input.Page, 1)
	diffs, response, err := client.api.MergeRequests.ListMergeRequestDiffs(id, iid,
		&gitlab.ListMergeRequestDiffsOptions{ListOptions: gitlab.ListOptions{PerPage: pageSize, Page: page}}, gitlab.WithContext(ctx))
	if err != nil {
		return result, err
	}
	nextPage := response.NextPage
	if nextPage == 0 && response.NextLink != "" {
		if next, err := url.Parse(response.NextLink); err == nil {
			nextPage, _ = strconv.ParseInt(next.Query().Get("page"), 10, 64)
		}
	}
	if (nextPage != 0 || response.NextLink != "") && nextPage <= page {
		return result, fmt.Errorf("GitLab returned an invalid diff continuation page")
	}
	result = mergeRequestDiffResult{Project: project, MRIID: iid, Truncated: nextPage != 0, NextPage: nextPage, Diffs: []*gitlab.MergeRequestDiff{}}
	for _, diff := range diffs {
		if input.FilePath == "" || diff.NewPath == input.FilePath || diff.OldPath == input.FilePath {
			result.Diffs = append(result.Diffs, diff)
		}
	}
	result.FileCount = len(result.Diffs)
	return result, nil
}

type mrCommentsInput struct {
	mrReadInput
	IncludeSystemNotes bool   `json:"include_system_notes,omitempty" jsonschema:"Include system-generated notes."`
	After              string `json:"after,omitempty" jsonschema:"Use next_cursor from the previous response to continue."`
}

type mergeRequestDiscussion struct {
	ID       string             `json:"id"`
	Resolved bool               `json:"resolved"`
	Notes    []mergeRequestNote `json:"notes"`
}

type mergeRequestNote struct {
	Author    string `json:"author"`
	CreatedAt string `json:"created_at"`
	Body      string `json:"body"`
	System    bool   `json:"system"`
	FilePath  string `json:"file_path,omitempty"`
	Line      *int   `json:"line,omitempty"`
}

type mergeRequestCommentsResult struct {
	Project         string                   `json:"project"`
	MRIID           int64                    `json:"mr_iid"`
	DiscussionCount int                      `json:"discussion_count"`
	Truncated       bool                     `json:"truncated"`
	NextCursor      string                   `json:"next_cursor,omitempty"`
	Discussions     []mergeRequestDiscussion `json:"discussions"`
}

func readMergeRequestComments(ctx context.Context, input mrCommentsInput) (mergeRequestCommentsResult, error) {
	var result mergeRequestCommentsResult
	project, iid, err := input.target()
	if err != nil {
		return result, err
	}
	client, err := newPublicMergeRequests()
	if err != nil {
		return result, err
	}
	path := project
	if id, err := strconv.ParseInt(project, 10, 64); err == nil && id > 0 {
		details, _, err := client.api.Projects.GetProject(id, nil, gitlab.WithContext(ctx))
		if err != nil {
			return result, err
		}
		path = details.PathWithNamespace
	}
	// Wikimedia GitLab serves public discussions through GraphQL.
	const query = `
query($fullPath: ID!, $iid: String!, $after: String) {
  project(fullPath: $fullPath) {
    mergeRequest(iid: $iid) {
      discussions(first: 100, after: $after) {
        pageInfo { hasNextPage endCursor }
        nodes {
          id
          resolved
          notes { nodes { author { username } createdAt body system position { newPath newLine oldPath oldLine } } }
        }
      }
    }
  }
}`
	var data struct {
		Project struct {
			MergeRequest *struct {
				Discussions struct {
					PageInfo struct {
						HasNextPage bool
						EndCursor   string
					}
					Nodes []struct {
						ID       string
						Resolved bool
						Notes    struct {
							Nodes []struct {
								Author    struct{ Username string }
								CreatedAt string
								Body      string
								System    bool
								Position  struct {
									NewPath string
									NewLine *int
									OldPath string
									OldLine *int
								}
							}
						}
					}
				}
			}
		}
	}
	variables := map[string]string{"fullPath": path, "iid": strconv.FormatInt(iid, 10)}
	if input.After != "" {
		variables["after"] = input.After
	}
	if err := gitLabGraphQL(ctx, client.http, query, variables, &data); err != nil {
		return result, err
	}
	if data.Project.MergeRequest == nil {
		return result, fmt.Errorf("public GitLab merge request %s!%d not found", project, iid)
	}
	discussions := data.Project.MergeRequest.Discussions
	result = mergeRequestCommentsResult{Project: project, MRIID: iid, Truncated: discussions.PageInfo.HasNextPage, Discussions: []mergeRequestDiscussion{}}
	if result.Truncated {
		result.NextCursor = discussions.PageInfo.EndCursor
		if result.NextCursor == "" || result.NextCursor == input.After {
			return mergeRequestCommentsResult{}, fmt.Errorf("GitLab returned an invalid discussion continuation cursor")
		}
	}
	for _, node := range discussions.Nodes {
		thread := mergeRequestDiscussion{ID: node.ID, Resolved: node.Resolved}
		for _, note := range node.Notes.Nodes {
			if note.System && !input.IncludeSystemNotes {
				continue
			}
			path, line := note.Position.NewPath, note.Position.NewLine
			if line == nil && note.Position.OldLine != nil {
				path, line = note.Position.OldPath, note.Position.OldLine
			}
			thread.Notes = append(thread.Notes, mergeRequestNote{Author: note.Author.Username, CreatedAt: note.CreatedAt, Body: note.Body, System: note.System, FilePath: path, Line: line})
		}
		if len(thread.Notes) > 0 {
			result.Discussions = append(result.Discussions, thread)
		}
	}
	result.DiscussionCount = len(result.Discussions)
	return result, nil
}
