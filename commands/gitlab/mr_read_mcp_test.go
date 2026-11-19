package gitlab

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type mcpTestTransport func(*http.Request) (*http.Response, error)

func (transport mcpTestTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestPublicMergeRequestReads(t *testing.T) {
	transport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = transport })
	http.DefaultTransport = mcpTestTransport(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet || request.URL.Scheme != "https" || request.URL.Host != publicGitLabHost ||
			request.Header.Get("Authorization") != "" || request.Header.Get("Private-Token") != "" || request.Header.Get("Cookie") != "" {
			t.Errorf("unexpected public GitLab request: %s %s", request.Method, request.URL)
		}
		var body string
		header := make(http.Header)
		switch request.URL.Path {
		case "/api/graphql":
			values := request.URL.Query()
			if strings.Contains(values.Get("query"), "discussions") {
				switch got := values.Get("variables"); got {
				case `{"fullPath":"repos/example/tools","iid":"17"}`:
					body = `{"data":{"project":{"mergeRequest":{"discussions":{"pageInfo":{"hasNextPage":true,"endCursor":"page-two"},"nodes":[{"id":"thread","notes":{"nodes":[{"system":true,"body":"approved"},{"body":"line comment","position":{"oldPath":"removed.go","oldLine":8}}]}}]}}}}}`
				case `{"after":"page-two","fullPath":"repos/example/tools","iid":"17"}`:
					body = `{"data":{"project":{"mergeRequest":{"discussions":{"pageInfo":{"hasNextPage":false},"nodes":[{"id":"later","notes":{"nodes":[{"body":"later comment"}]}}]}}}}}`
				default:
					t.Errorf("discussion variables = %s", got)
				}
			} else {
				if got := values.Get("variables"); got != `{"fullPath":"repos/example/tools"}` {
					t.Errorf("lookup variables = %s", got)
				}
				body = `{"data":{"project":{"id":"gid://gitlab/Project/29"}}}`
			}
		case "/api/v4/projects/29/merge_requests/17":
			body = `{"iid":17,"title":"Public merge request"}`
		case "/api/v4/projects/29/merge_requests/17/diffs":
			switch page := request.URL.Query().Get("page"); page {
			case "", "1":
				header.Set("Link", "<https://gitlab.wikimedia.org/api/v4/projects/29/merge_requests/17/diffs?page=2>; rel=\"next\"")
				body = `[{"new_path":"other.go"}]`
			case "2":
				body = `[{"old_path":"old.go","new_path":"new.go","diff":"a diff"}]`
			default:
				t.Errorf("unexpected diff page %q", page)
			}
		default:
			t.Errorf("unexpected path %q", request.URL.EscapedPath())
		}
		return &http.Response{StatusCode: http.StatusOK, Header: header, Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
	})
	input := mrReadInput{MRURL: "https://gitlab.wikimedia.org/repos/example/tools/-/merge_requests/17#note_42"}
	mr, err := readMergeRequest(t.Context(), input)
	if err != nil || mr.IID != 17 || mr.Title != "Public merge request" {
		t.Fatalf("merge request = %+v, %v", mr, err)
	}
	diff, err := readMergeRequestDiff(t.Context(), mrDiffInput{mrReadInput: input, FilePath: "old.go"})
	if err != nil || diff.FileCount != 0 || !diff.Truncated || diff.NextPage != 2 {
		t.Fatalf("first diff page = %+v, %v", diff, err)
	}
	diff, err = readMergeRequestDiff(t.Context(), mrDiffInput{mrReadInput: input, FilePath: "old.go", Page: diff.NextPage})
	if err != nil || diff.Truncated || diff.FileCount != 1 || diff.Diffs[0].NewPath != "new.go" {
		t.Fatalf("diff = %+v, %v", diff, err)
	}
	comments, err := readMergeRequestComments(t.Context(), mrCommentsInput{mrReadInput: input})
	if err != nil || !comments.Truncated || comments.NextCursor != "page-two" || comments.DiscussionCount != 1 || len(comments.Discussions[0].Notes) != 1 {
		t.Fatalf("comments = %+v, %v", comments, err)
	}
	note := comments.Discussions[0].Notes[0]
	if note.FilePath != "removed.go" || note.Line == nil || *note.Line != 8 {
		t.Fatalf("note position = %+v", note)
	}
	comments, err = readMergeRequestComments(t.Context(), mrCommentsInput{mrReadInput: input, IncludeSystemNotes: true})
	if err != nil || comments.DiscussionCount != 1 || len(comments.Discussions[0].Notes) != 2 {
		t.Fatalf("comments including system notes = %+v, %v", comments, err)
	}
	comments, err = readMergeRequestComments(t.Context(), mrCommentsInput{mrReadInput: input, After: comments.NextCursor})
	if err != nil || comments.Truncated || comments.DiscussionCount != 1 || comments.Discussions[0].ID != "later" {
		t.Fatalf("next comments page = %+v, %v", comments, err)
	}
	if _, err := readMergeRequest(t.Context(), mrReadInput{MRURL: "https://example.org/repos/example/tools/-/merge_requests/17"}); err == nil {
		t.Fatal("accepted a foreign merge request URL")
	}
}
