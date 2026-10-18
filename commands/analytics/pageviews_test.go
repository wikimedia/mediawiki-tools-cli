package analytics

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestPageviewsSharedOutputFormats(t *testing.T) {
	previous := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = previous })
	http.DefaultClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"items":[{"project":"en.wikipedia.org","article":"Albert, Einstein","timestamp":"2026091800","views":12261},{"project":"en.wikipedia.org","article":"Albert, Einstein","timestamp":"2026091900","views":10997}]}`)),
		}, nil
	})}
	for _, format := range []string{"table", "json", "jq", "csv"} {
		t.Run(format, func(t *testing.T) {
			cmd := NewPageviewsCmd()
			var buffer bytes.Buffer
			cmd.SetOut(&buffer)
			args := []string{"Albert, Einstein", "--output", format}
			if format == "jq" {
				args = append(args, "--format", ".[].views")
			}
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			switch format {
			case "table":
				if got, want := strings.Fields(buffer.String()), []string{"Date", "Views", "2026-09-18", "12261", "2026-09-19", "10997"}; !reflect.DeepEqual(got, want) {
					t.Fatalf("table dates, counts, or row order differ: %q", buffer.String())
				}
			case "json":
				var rows []pageviewItem
				if err := json.Unmarshal(buffer.Bytes(), &rows); err != nil {
					t.Fatal(err)
				}
				if len(rows) != 2 || rows[0].Project != "en.wikipedia.org" || rows[0].Article != "Albert, Einstein" || rows[0].Views != 12261 || rows[1].Timestamp != "2026091900" {
					t.Fatalf("JSON lost pageview data: %s", buffer.String())
				}
			case "jq":
				if buffer.String() != "12261\n10997\n" {
					t.Fatalf("jq expression not applied: %q", buffer.String())
				}
			case "csv":
				rows, err := csv.NewReader(&buffer).ReadAll()
				if err != nil {
					t.Fatal(err)
				}
				if len(rows) != 3 || !reflect.DeepEqual(rows[0], []string{"timestamp", "project", "article", "views"}) || rows[1][2] != "Albert, Einstein" || rows[1][3] != "12261" {
					t.Fatalf("CSV lost or failed to quote pageview data: %#v", rows)
				}
			}
		})
	}
}

func TestPageviewsOutputValidationBeforeRequest(t *testing.T) {
	previous := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = previous })
	http.DefaultClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Fatal("invalid output options made an API request")
		return nil, nil
	})}
	for _, args := range [][]string{{"Albert Einstein", "--output", "invalid"}, {"Albert Einstein", "--output", "jq"}} {
		cmd := NewPageviewsCmd()
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Fatalf("invalid options accepted: %v", args)
		}
	}
	cmd := NewPageviewsCmd()
	if cmd.Flags().Lookup("output").DefValue != "auto" || cmd.Flags().Lookup("format").DefValue != "" {
		t.Fatal("command does not use the shared automatic output defaults")
	}
}
