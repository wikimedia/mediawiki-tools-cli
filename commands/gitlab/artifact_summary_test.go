package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	"gitlab.com/gitlab-org/cli/api"
)

func TestArtifactStorageAggregation(t *testing.T) {
	summary := newArtifactSummary()
	for _, artifact := range []ciArtifact{
		{JobID: "1", JobName: "build", Type: "archive", Size: 1024},
		{JobID: "1", JobName: "build", Type: "metadata", Size: 100},
		{JobID: "2", JobName: "build", Type: "ARCHIVE", Size: 2048},
		{JobID: "3", JobName: "test", Type: "junit", Size: 100},
		{JobID: "4", JobName: "other", Size: 100},
	} {
		if err := summary.add(artifact); err != nil {
			t.Fatal(err)
		}
	}
	got := summary.result()
	if got.Total != (artifactUsage{Jobs: 4, Artifacts: 5, SizeBytes: 3372}) {
		t.Fatalf("total = %+v", got.Total)
	}
	if len(got.ByJob) != 3 || got.ByJob[0].Name != "build" || got.ByJob[0].Jobs != 2 || got.ByJob[0].Artifacts != 3 || got.ByJob[0].SizeBytes != 3172 {
		t.Fatalf("job groups = %+v", got.ByJob)
	}
	if got.ByJob[1].Name != "other" || got.ByJob[2].Name != "test" {
		t.Fatalf("tie order = %+v", got.ByJob)
	}
	if len(got.ByType) != 4 || got.ByType[0].Name != "archive" || got.ByType[0].SizeBytes != 3072 || got.ByType[0].Jobs != 2 {
		t.Fatalf("type groups = %+v", got.ByType)
	}
	if got.ByType[1].Name != "junit" || got.ByType[2].Name != "metadata" || got.ByType[3].Name != "unknown" {
		t.Fatalf("type tie order = %+v", got.ByType)
	}
}

func TestArtifactSummaryInvalidSize(t *testing.T) {
	summary := newArtifactSummary()
	if err := summary.add(ciArtifact{Size: -1}); err == nil {
		t.Fatal("negative size accepted")
	}
	if err := summary.add(ciArtifact{Size: math.MaxInt64}); err != nil {
		t.Fatal(err)
	}
	if err := summary.add(ciArtifact{Size: 1}); err == nil {
		t.Fatal("overflow accepted")
	}
}

func TestArtifactSummaryOutput(t *testing.T) {
	for _, empty := range []bool{false, true} {
		summary := newArtifactSummary()
		if !empty {
			if err := summary.add(ciArtifact{JobID: "1", JobName: "build", Type: "archive", Size: 1024}); err != nil {
				t.Fatal(err)
			}
		}
		cmd := &cobra.Command{}
		var out bytes.Buffer
		cmd.SetOut(&out)
		if err := outputArtifactSummary(cmd, "json", summary.result()); err != nil {
			t.Fatal(err)
		}
		var got artifactStorageSummary
		if err := json.Unmarshal(out.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Total != summary.result().Total || got.ByJob == nil || got.ByType == nil {
			t.Fatalf("JSON = %s", out.String())
		}
		out.Reset()
		if err := outputArtifactSummary(cmd, "table", summary.result()); err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"Total:", "JOB NAME", "ARTIFACT TYPE", "JOBS", "ARTIFACTS", "SIZE", "job logs excluded"} {
			if !strings.Contains(out.String(), want) {
				t.Fatalf("missing %q: %s", want, out.String())
			}
		}
		if !empty && !strings.Contains(out.String(), "1.0 KiB in 1 artifacts across 1 jobs") {
			t.Fatalf("text = %s", out.String())
		}
	}
}

func artifactTestClient(t *testing.T, handler http.HandlerFunc) *gitlab.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := gitlab.NewClient("", gitlab.WithBaseURL(server.URL+"/api/v4/"), gitlab.WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func artifactScanCommand() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	return cmd
}

func TestArtifactSummaryPaginationAndFilters(t *testing.T) {
	for _, filtered := range []bool{false, true} {
		t.Run(fmt.Sprintf("filtered_%t", filtered), func(t *testing.T) {
			pages := 0
			client := artifactTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				pages++
				if r.Method != http.MethodGet || r.URL.Path != "/api/v4/projects/16/jobs" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				if r.URL.Query().Get("pagination") != "keyset" || r.URL.Query().Get("include_retried") != "true" {
					t.Errorf("pagination options = %s", r.URL.RawQuery)
				}
				if pages == 1 {
					w.Header().Set("Link", fmt.Sprintf(`<http://%s/api/v4/projects/16/jobs?pagination=keyset&include_retried=true&id_before=2>; rel="next"`, r.Host))
					fmt.Fprint(w, `[{"id":2,"name":"build","ref":"main","created_at":"2026-10-31T00:00:00Z","artifacts_file":{"size":1024},"artifacts":[{"file_type":"archive","size":1024},{"file_type":"metadata","size":128},{"file_type":"trace","size":99999}]}]`)
				} else {
					if r.URL.Query().Get("id_before") != "2" {
						t.Errorf("next page = %s", r.URL)
					}
					fmt.Fprint(w, `[{"id":1,"name":"build","ref":"refs/merge-requests/1/head","created_at":"2020-01-01T00:00:00Z","artifacts":[{"file_type":"archive","size":2048},{"file_type":"junit","size":100}]}]`)
				}
			})
			summary := newArtifactSummary()
			refPrefix := ""
			minSize := int64(0)
			if filtered {
				refPrefix, minSize = "refs/merge-requests/", 1024
			}
			if err := scanCIArtifacts(artifactScanCommand(), client, 16, time.Time{}, minSize, refPrefix, 0, summary.add); err != nil {
				t.Fatal(err)
			}
			want := artifactUsage{Jobs: 2, Artifacts: 4, SizeBytes: 3300}
			if filtered {
				want = artifactUsage{Jobs: 1, Artifacts: 1, SizeBytes: 2048}
			}
			if pages != 2 || summary.result().Total != want {
				t.Fatalf("pages = %d, total = %+v, want %+v", pages, summary.result().Total, want)
			}
		})
	}
}

func TestArtifactSummaryScansBeyondListSafetyLimit(t *testing.T) {
	pages := 0
	client := artifactTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		pages++
		nextID := maxArtifactJobs + 1
		if cursor := r.URL.Query().Get("id_before"); cursor != "" {
			value, err := strconv.Atoi(cursor)
			if err != nil {
				t.Error(err)
				return
			}
			nextID = value - 1
		}
		count := min(nextID, 100)
		jobs := make([]map[string]interface{}, count)
		for i := range jobs {
			jobs[i] = map[string]interface{}{"id": nextID - i, "created_at": "2026-10-31T00:00:00Z"}
		}
		lastID := nextID - count + 1
		if lastID > 1 {
			w.Header().Set("Link", fmt.Sprintf(`<http://%s/api/v4/projects/16/jobs?pagination=keyset&id_before=%d>; rel="next"`, r.Host, lastID))
		} else {
			jobs[len(jobs)-1]["artifacts"] = []map[string]interface{}{{"file_type": "archive", "size": 4096}}
		}
		if err := json.NewEncoder(w).Encode(jobs); err != nil {
			t.Error(err)
		}
	})
	summary := newArtifactSummary()
	if err := scanCIArtifacts(artifactScanCommand(), client, 16, time.Time{}, 0, "", 0, summary.add); err != nil {
		t.Fatal(err)
	}
	if summary.result().Total.SizeBytes != 4096 || pages != 101 {
		t.Fatalf("old artifact not counted: %+v, pages = %d", summary.result().Total, pages)
	}
	if artifacts, err := listCIArtifacts(artifactScanCommand(), client, 16, time.Time{}, 0, ""); err == nil || !strings.Contains(err.Error(), "10000-job safety limit") || artifacts != nil {
		t.Fatalf("list safety limit changed: artifacts = %+v, error = %v", artifacts, err)
	}
}

func TestArtifactScanErrorsAndAgeWindow(t *testing.T) {
	client := artifactTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"id":2,"created_at":"2026-10-31T00:00:00Z","artifacts":[{"size":1024}]},{"id":1,"created_at":"2020-01-01T00:00:00Z","artifacts":[{"size":2048}]}]`)
	})
	summary := newArtifactSummary()
	cutoff := time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)
	if err := scanCIArtifacts(artifactScanCommand(), client, 16, cutoff, 0, "", 0, summary.add); err != nil {
		t.Fatal(err)
	}
	if summary.result().Total.SizeBytes != 1024 {
		t.Fatalf("age filter ignored: %+v", summary.result().Total)
	}
	if _, err := listCIArtifacts(artifactScanCommand(), client, 16, cutoff, 0, ""); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cmd := &cobra.Command{}
	cmd.SetContext(ctx)
	if err := scanCIArtifacts(cmd, client, 16, time.Time{}, 0, "", 0, summary.add); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}
	missingDate := artifactTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `[{"id":1,"artifacts":[{"size":1024}]}]`)
	})
	if err := scanCIArtifacts(artifactScanCommand(), missingDate, 16, cutoff, 0, "", 0, summary.add); err == nil || !strings.Contains(err.Error(), "creation timestamp") {
		t.Fatalf("missing timestamp error = %v", err)
	}
	failing := artifactTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	if err := scanCIArtifacts(artifactScanCommand(), failing, 16, time.Time{}, 0, "", 0, summary.add); err == nil {
		t.Fatal("API failure accepted")
	}
	pages := 0
	laterFailure := artifactTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		pages++
		if pages == 1 {
			w.Header().Set("Link", fmt.Sprintf(`<http://%s/api/v4/projects/16/jobs?id_before=2>; rel="next"`, r.Host))
			fmt.Fprint(w, `[{"id":2,"artifacts":[{"size":1024}]}]`)
		} else {
			w.WriteHeader(http.StatusForbidden)
		}
	})
	if err := scanCIArtifacts(artifactScanCommand(), laterFailure, 16, time.Time{}, 0, "", 0, newArtifactSummary().add); err == nil || pages != 2 {
		t.Fatalf("later-page failure ignored: pages = %d, error = %v", pages, err)
	}
}

func TestArtifactSummaryCommand(t *testing.T) {
	factory := testFactory(t)
	factory.RepoOverride("gitlab.wikimedia.org/repos/releng/cli")
	api.RefreshClient()
	t.Cleanup(api.RefreshClient)
	_, err := api.TestClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet {
			t.Errorf("summary attempted %s", r.Method)
		}
		body := `{"id":16,"path_with_namespace":"repos/releng/cli"}`
		if strings.HasSuffix(r.URL.Path, "/jobs") {
			body = `[{"id":1,"name":"build","artifacts_file":{"size":1024},"artifacts":[{"file_type":"archive","size":1024},{"file_type":"metadata","size":128},{"file_type":"trace","size":999}]}]`
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})}, "", "gitlab.wikimedia.org", false)
	if err != nil {
		t.Fatal(err)
	}
	cmd := newCIArtifactsCommand(factory)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"summary", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got artifactStorageSummary
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Total != (artifactUsage{Jobs: 1, Artifacts: 2, SizeBytes: 1152}) {
		t.Fatalf("command output = %s", out.String())
	}
	for _, unchanged := range []*cobra.Command{newCIArtifactsListCommand(factory), newCIArtifactsPruneCommand(factory)} {
		if unchanged.Flags().Lookup("since").DefValue != "48h" {
			t.Fatalf("%s default changed", unchanged.Name())
		}
	}
	for _, args := range [][]string{{"--since", "0h"}, {"--since", "bad"}, {"--min-size", "bad"}, {"--output", "bad"}, {"--yes"}, {"extra"}} {
		cmd := newCIArtifactsSummaryCommand(factory)
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Fatalf("invalid args accepted: %v", args)
		}
	}
}
