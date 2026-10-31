package gitlab

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

func TestListCIArtifactsPreservesInt64IDs(t *testing.T) {
	const largeID int64 = 9007199254740993
	client, err := gitlab.NewClient("test-token", gitlab.WithHTTPClient(&http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.Path != "/api/v4/projects/9007199254740993/jobs" {
				t.Errorf("jobs URL = %q", request.URL.Path)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`[{
					"id": 9007199254740993,
					"name": "build", "ref": "main", "created_at": "2026-01-02T00:00:00Z",
					"artifacts": [{"filename": "binary.zip", "file_type": "archive", "size": 1024}]
				}]`)),
				Request: request,
			}, nil
		}),
	}))
	if err != nil {
		t.Fatal(err)
	}
	cmd := &cobra.Command{}
	cmd.SetErr(io.Discard)
	artifacts, err := listCIArtifacts(cmd, client, largeID, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 1 || artifacts[0].JobID != "9007199254740993" {
		t.Fatalf("artifacts = %+v", artifacts)
	}
	query, err := bulkDestroyQuery(largeID, []string{"gid://gitlab/Ci::JobArtifact/42"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(query, `"gid://gitlab/Project/9007199254740993"`) {
		t.Fatalf("project ID missing from mutation: %s", query)
	}
}
