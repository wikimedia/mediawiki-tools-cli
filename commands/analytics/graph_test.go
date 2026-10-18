package analytics

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestAnalyticsGraphOutput(t *testing.T) {
	previous := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = previous })
	for _, tc := range []struct {
		name, body, want string
		args             []string
	}{
		{"article views", `{"items":[{"timestamp":"2025010100","views":42}]}`, "1 observation", []string{"pageviews", "Albert Einstein"}},
		{"total views", `{"items":[{"timestamp":"2025010100","views":42}]}`, "1 observation", []string{"pageviews"}},
		{"edits", `{"items":[{"results":[{"timestamp":"2025-01-01T00:00:00Z","edits":42}]}]}`, "1 observation", []string{"edits", "Albert Einstein"}},
		{"editors", `{"items":[{"results":[{"timestamp":"2025-01-01T00:00:00Z","editors":42}]}]}`, "1 observation", []string{"editors"}},
		{"new pages", `{"items":[{"results":[{"timestamp":"2025-01-01T00:00:00Z","new_pages":42}]}]}`, "1 observation", []string{"new-pages"}},
		{"devices", `{"items":[{"timestamp":"20250101","devices":42}]}`, "1 observation", []string{"devices"}},
		{"requests", `{"items":[{"timestamp":"2025010100","requests":42}]}`, "1 observation", []string{"mediarequests"}},
		{"ranked views", `{"items":[{"articles":[{"rank":1,"article":"First","views":42},{"rank":2,"article":"Second","views":20}]}]}`, "First", []string{"top-pages", "--limit", "1"}},
		{"ranked edits", `{"items":[{"results":[{"top":[{"rank":1,"page_title":"First","edits":42}]}]}]}`, "First", []string{"top-edited"}},
		{"ranked requests", `{"items":[{"files":[{"rank":1,"file_path":"First","requests":42}]}]}`, "First", []string{"mediarequests", "top"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			http.DefaultClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			})}
			cmd := NewCmd()
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetArgs(append(tc.args, "--output", "graph", "--graph-width", "50"))
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if calls != 1 || !strings.Contains(buf.String(), tc.want) || strings.Contains(buf.String(), "Second") {
				t.Fatalf("unexpected graph: %s (requests %d)", buf.String(), calls)
			}
			if !strings.Contains(buf.String(), "●") && !strings.Contains(buf.String(), "█") {
				t.Fatal("missing graph")
			}
		})
	}
}

func TestGraphWidthValidatedBeforeRequest(t *testing.T) {
	previous := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = previous })
	http.DefaultClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { t.Fatal("invalid width made request"); return nil, nil })}
	for _, command := range []string{"pageviews", "edits", "top-pages"} {
		cmd := NewCmd()
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{command, "--output", "graph", "--graph-width", "10"})
		if err := cmd.Execute(); err == nil {
			t.Fatal("invalid width accepted")
		}
	}
}

func TestGraphRejectsMalformedObservations(t *testing.T) {
	previous := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = previous })
	for _, body := range []string{
		`{"items":[{"timestamp":"invalid","devices":42}]}`,
		`{"items":[{"timestamp":"20250101","devices":"unknown"}]}`,
	} {
		http.DefaultClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
		})}
		cmd := NewCmd()
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"devices", "--output", "graph"})
		if err := cmd.Execute(); err == nil {
			t.Fatal("invalid observation graphed")
		}
	}
}

func TestRankingGraphOffsetRescalesRemainingResults(t *testing.T) {
	previous := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = previous })
	http.DefaultClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		body := `{"items":[{"articles":[{"rank":1,"article":"Main_Page","views":10000000},{"rank":2,"article":"Second","views":100},{"rank":3,"article":"Third","views":50}]}]}`
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	cmd := NewCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"top-pages", "--date", "20250101", "--offset", "1", "--limit", "1", "--output", "graph", "--graph-width", "50"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "Main_Page") || strings.Contains(buf.String(), "Third") || !strings.Contains(buf.String(), "Second") {
		t.Fatalf("incorrect selection: %s", buf.String())
	}
	if strings.Count(buf.String(), "█") < 30 {
		t.Fatalf("graph still scaled against skipped page: %s", buf.String())
	}
}
