package gerrit

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type reviewTestTransport func(*http.Request) (*http.Response, error)

func (transport reviewTestTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestReviewToolsUseSelectedPatchSet(t *testing.T) {
	old := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = old })
	const revision = "0123456789abcdef0123456789abcdef01234567"
	prefix := "/r/changes/42/revisions/" + revision
	responses := map[string]string{
		prefix + "/files/":                    `{"/COMMIT_MSG":{},"dir/file.go":{}}`,
		prefix + "/files/%2FCOMMIT_MSG/diff/": `{"content":[]}`,
		prefix + "/files/dir%2Ffile.go/diff/": `{"content":[]}`,
		prefix + "/comments/":                 `{}`,
		prefix + "/commit/":                   `{"message":"Title\n\nChange-Id: I123\n"}`,
	}
	calls := 0
	http.DefaultTransport = reviewTestTransport(func(request *http.Request) (*http.Response, error) {
		calls++
		body, ok := responses[request.URL.EscapedPath()]
		if !ok || request.URL.Host != "gerrit.wikimedia.org" || request.Method != http.MethodGet || request.Header.Get("Authorization") != "" {
			t.Errorf("unexpected request %s %s", request.Method, request.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(")]}'\n" + body)), Request: request}, nil
	})
	input := revisionReadInput{changeReadInput: changeReadInput{ChangeID: "42"}, Revision: revision}
	if _, err := listChangeFiles(t.Context(), input); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"/COMMIT_MSG", "dir/file.go"} {
		if _, err := fileDiff(t.Context(), fileDiffInput{revisionReadInput: input, FilePath: file}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := listChangeComments(t.Context(), input); err != nil {
		t.Fatal(err)
	}
	message, err := commitMessage(t.Context(), input)
	if err != nil || message["verbatim_commit_message"] != "Title\n\nChange-Id: I123\n" {
		t.Fatalf("commit message = %v, %v", message, err)
	}
	if _, err := fileDiff(t.Context(), fileDiffInput{revisionReadInput: input, FilePath: "/unexpected"}); err == nil {
		t.Fatal("accepted an unknown absolute path")
	}
	input.Revision = ".."
	if _, err := listChangeFiles(t.Context(), input); err == nil {
		t.Fatal("accepted an invalid revision")
	}
	if calls != len(responses) {
		t.Fatalf("made %d requests, want %d", calls, len(responses))
	}
}
