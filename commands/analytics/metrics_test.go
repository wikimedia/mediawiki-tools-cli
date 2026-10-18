package analytics

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestMetricCommandsUseCorrectRoutesAndPreserveContext(t *testing.T) {
	previous := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = previous })
	for _, tc := range []struct {
		name, path, body, field string
		args                    []string
	}{
		{"pageviews totals", "/metrics/pageviews/aggregate/en.wikipedia.org/all-access/all-agents/daily/2025010100/2025010200", `{"items":[{"project":"en.wikipedia","timestamp":"2025010100","views":42}]}`, "views", []string{"pageviews"}},
		{"page edits", "/metrics/edits/per-page/en.wikipedia.org/A%2FB/user/daily/20250101/20250103", `{"items":[{"project":"en.wikipedia","page-title":"A/B","results":[{"timestamp":"2025-01-01T00:00:00Z","edits":42}]}]}`, "edits", []string{"edits", "A/B", "--editor-type", "user"}},
		{"project edits", "/metrics/edits/aggregate/en.wikipedia.org/all-editor-types/all-page-types/daily/20250101/20250103", `{"items":[{"project":"en.wikipedia","results":[{"timestamp":"2025-01-01T00:00:00Z","edits":42}]}]}`, "edits", []string{"edits"}},
		{"editors", "/metrics/editors/aggregate/en.wikipedia.org/all-editor-types/all-page-types/5..24-edits/daily/20250101/20250103", `{"items":[{"project":"en.wikipedia","results":[{"timestamp":"2025-01-01T00:00:00Z","editors":42}]}]}`, "editors", []string{"editors", "--activity-level", "5..24-edits"}},
		{"new pages", "/metrics/edited-pages/new/en.wikipedia.org/all-editor-types/content/daily/20250101/20250103", `{"items":[{"project":"en.wikipedia","results":[{"timestamp":"2025-01-01T00:00:00Z","new_pages":42}]}]}`, "new_pages", []string{"new-pages", "--page-type", "content"}},
		{"devices", "/metrics/unique-devices/en.wikipedia.org/mobile-site/daily/20250101/20250102", `{"items":[{"project":"en.wikipedia","timestamp":"20250101","devices":42}]}`, "devices", []string{"devices", "--access-site", "mobile-site"}},
		{"media requests", "/metrics/mediarequests/aggregate/all-referers/audio/user/daily/20250101/20250102", `{"items":[{"referer":"all-referers","timestamp":"2025010100","requests":42}]}`, "requests", []string{"mediarequests", "--media-type", "audio", "--agent", "user"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			http.DefaultClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.EscapedPath() != "/api/rest_v1"+tc.path {
					t.Errorf("route: %s", r.URL.EscapedPath())
				}
				if r.Header.Get("User-Agent") == "" {
					t.Error("missing user agent")
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			})}
			cmd := NewCmd()
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetArgs(append(tc.args, "--start", "20250101", "--end", "20250102", "--output", "json"))
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			var rows []map[string]interface{}
			if err := json.Unmarshal(buf.Bytes(), &rows); err != nil {
				t.Fatal(err)
			}
			if len(rows) != 1 || rows[0][tc.field] != float64(42) {
				t.Fatalf("unexpected rows: %s", buf.String())
			}
			if _, ok := rows[0]["results"]; ok {
				t.Fatal("time series not expanded into rows")
			}
			if tc.name != "media requests" && rows[0]["project"] != "en.wikipedia" {
				t.Fatal("project context lost")
			}
		})
	}
}

func TestRankingsUnwrapNestedResponsesAndApplyLimits(t *testing.T) {
	previous := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = previous })
	for _, tc := range []struct {
		name, path, body, field string
		args                    []string
	}{
		{"top pages", "/metrics/pageviews/top/en.wikipedia.org/all-access/2025/01/01", `{"items":[{"project":"en.wikipedia","articles":[{"rank":1,"article":"First","views":42},{"rank":2,"article":"Second","views":20}]}]}`, "article", []string{"top-pages", "--date", "20250101"}},
		{"top edited", "/metrics/edited-pages/top-by-edits/en.wikipedia.org/all-editor-types/all-page-types/2025/01/all-days", `{"items":[{"project":"en.wikipedia","results":[{"timestamp":"2025-01-01T00:00:00Z","top":[{"rank":1,"page_title":"First","edits":42},{"rank":2,"page_title":"Second","edits":20}]}]}]}`, "page_title", []string{"top-edited", "--month", "202501"}},
		{"top files", "/metrics/mediarequests/top/all-referers/all-media-types/2025/01/01", `{"items":[{"referer":"all-referers","files":[{"rank":1,"file_path":"First","requests":42},{"rank":2,"file_path":"Second","requests":20}]}]}`, "file_path", []string{"mediarequests", "top", "--date", "20250101"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			http.DefaultClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.EscapedPath() != "/api/rest_v1"+tc.path {
					t.Errorf("route: %s", r.URL.EscapedPath())
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			})}
			for _, limit := range []string{"1", "0"} {
				cmd := NewCmd()
				var buf bytes.Buffer
				cmd.SetOut(&buf)
				cmd.SetArgs(append(append([]string{}, tc.args...), "--limit", limit, "--output", "json"))
				if err := cmd.Execute(); err != nil {
					t.Fatal(err)
				}
				var rows []map[string]interface{}
				if err := json.Unmarshal(buf.Bytes(), &rows); err != nil {
					t.Fatal(err)
				}
				expected := 1
				if limit == "0" {
					expected = 2
				}
				if len(rows) != expected || rows[0][tc.field] != "First" {
					t.Fatalf("ranking or limit lost: %s", buf.String())
				}
			}
		})
	}
}

func TestMetricValidationPreventsRequests(t *testing.T) {
	previous := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = previous })
	http.DefaultClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Fatal("invalid options made a request")
		return nil, nil
	})}
	for _, args := range [][]string{
		{"top-pages", "--date", "20250230"},
		{"top-edited", "--month", "202513"},
		{"top-pages", "--date", "20250101", "--month", "202501"},
		{"top-pages", "--limit", "-1"},
		{"devices", "--access-site", "mobile-app"},
		{"editors", "--activity-level", "invalid"},
		{"edits", "Article", "--page-type", "content"},
		{"edits", "--editor-type", "robot"},
		{"new-pages", "--start", "20250102", "--end", "20250101"},
		{"mediarequests", "--agent", "automated"},
		{"mediarequests", "--media-type", "invalid"},
		{"devices", "--granularity", "hourly"},
	} {
		cmd := NewCmd()
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Errorf("invalid options accepted: %v", args)
		}
	}
	if rows, err := metricRows(nil); err != nil || rows == nil || len(rows) != 0 {
		t.Fatal("empty results must be an empty array")
	}
	if _, err := metricRows([]map[string]interface{}{{"results": "invalid"}}); err == nil {
		t.Fatal("invalid response silently accepted")
	}
}

func TestMetricTableCSVAndJQUseExpandedRows(t *testing.T) {
	previous := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = previous })
	http.DefaultClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"items":[{"project":"en.wikipedia","results":[{"timestamp":"2025-01-01T00:00:00.000Z","new_pages":42}]}]}`))}, nil
	})}
	for _, format := range []string{"table", "csv", "jq"} {
		cmd := NewCmd()
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		args := []string{"new-pages", "--output", format, "--start", "20250101", "--end", "20250101"}
		if format == "jq" {
			args = append(args, "--format", ".[].new_pages")
		}
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		switch format {
		case "table":
			if !strings.Contains(buf.String(), "2025-01-01") || !strings.Contains(buf.String(), "42") || strings.Contains(buf.String(), "T00:") {
				t.Fatalf("bad table date or count: %q", buf.String())
			}
		case "csv":
			if buf.String() != "timestamp,new_pages\n2025-01-01T00:00:00.000Z,42\n" {
				t.Fatalf("bad CSV fields: %q", buf.String())
			}
		case "jq":
			if buf.String() != "42\n" {
				t.Fatalf("jq did not see flattened rows: %q", buf.String())
			}
		}
	}
}
