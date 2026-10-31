package gitlab

import (
	"bytes"
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
	cmd.SetContext(t.Context())
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

func TestArtifactPruneSummary(t *testing.T) {
	artifacts := []ciArtifact{{ID: "first", Size: 1 << 30}, {ID: "second", Size: 512 << 20}, {ID: "third", Size: 256 << 20}}
	for _, tc := range []struct {
		name  string
		ids   []string
		count int
		want  string
	}{
		{"all", []string{"first", "second", "third"}, 3, "Deleted 3 of 3 selected artifacts (1.8 GiB).\n"},
		{"partial", []string{"second"}, 1, "Deleted 1 of 3 selected artifacts (512.0 MiB).\n"},
		{"none", nil, 0, "Deleted 0 of 3 selected artifacts (0 B).\n"},
		{"duplicate response IDs", []string{"first", "first"}, 1, "Deleted 1 of 3 selected artifacts (1.0 GiB).\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := outputCIArtifactsPruneSummary(&out, artifacts, tc.count, tc.ids); err != nil {
				t.Fatal(err)
			}
			if out.String() != tc.want {
				t.Fatalf("summary = %q, want %q", out.String(), tc.want)
			}
		})
	}
}

func TestFormatArtifactSize(t *testing.T) {
	for _, tc := range []struct {
		size int64
		want string
	}{{0, "0 B"}, {999, "999 B"}, {1024, "1.0 KiB"}, {150 << 20, "150.0 MiB"}, {1 << 40, "1.0 TiB"}} {
		if got := formatArtifactSize(tc.size); got != tc.want {
			t.Errorf("formatArtifactSize(%d) = %q, want %q", tc.size, got, tc.want)
		}
	}
}
