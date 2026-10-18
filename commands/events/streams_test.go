package events

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

const discoveryFixture = `{"paths": {
 "/v2/stream/{streams}": {"get":{"summary":"Subscribe"}},
 "/health": {"get":{}},
 "/v2/stream/revision-create": {"get":{"summary":"Revision events"}},
 "/v2/stream/recentchange": {"get":{
  "description":"Recent changes.\n\nSchema title: mediawiki/recentchange",
  "responses":{"200":{"content":{"application/json":{
   "schema":{"type":"object","properties":{"id":{"type":"integer"}}},
   "example":{"id":9007199254740993}
  }}}}
 }}
}}`

func mockEventHTTP(t *testing.T, handler eventTransport) {
	t.Helper()
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	http.DefaultTransport = handler
}

func discoveryHTTP(t *testing.T, status int, body string) {
	t.Helper()
	mockEventHTTP(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != eventStreamsURL+"/?spec" || req.Header.Get("User-Agent") != eventStreamsUserAgent {
			t.Fatalf("unexpected discovery request: %v", req)
		}
		return &http.Response{StatusCode: status, Status: http.StatusText(status), Body: io.NopCloser(strings.NewReader(body))}, nil
	})
}

func TestListStreams(t *testing.T) {
	for _, format := range []string{"json", "table"} {
		t.Run(format, func(t *testing.T) {
			discoveryHTTP(t, 200, discoveryFixture)
			var out bytes.Buffer
			cmd := NewListCmd()
			cmd.SetOut(&out)
			cmd.SetArgs([]string{"--output", format})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(out.String(), "{streams}") || strings.Contains(out.String(), "Schema title") {
				t.Fatalf("list should show only public stream summaries: %s", &out)
			}
			if format == "json" {
				var items []streamInfo
				if err := json.Unmarshal(out.Bytes(), &items); err != nil {
					t.Fatal(err)
				}
				if len(items) != 2 || items[0].Name != "recentchange" || items[1].Name != "revision-create" {
					t.Fatalf("unexpected stream list: %+v", items)
				}
			} else if !strings.Contains(out.String(), "Recent changes.") || !strings.Contains(out.String(), "Revision events") {
				t.Fatalf("missing table descriptions: %s", &out)
			}
		})
	}
}

func TestInfoStream(t *testing.T) {
	discoveryHTTP(t, 200, discoveryFixture)
	var out bytes.Buffer
	cmd := NewInfoCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"recentchange"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var info streamInfo
	if err := json.Unmarshal(out.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if info.Name != "recentchange" || !strings.Contains(info.Description, "Schema title") || len(info.Schema) == 0 || string(info.Example) != `{"id":9007199254740993}` {
		t.Fatalf("missing stream details or lost example precision: %s", &out)
	}
}

func TestDiscoveryFailures(t *testing.T) {
	for _, tt := range []struct {
		name   string
		status int
		body   string
		stream string
		want   string
	}{
		{"http", 503, "unavailable", "recentchange", "Service Unavailable"},
		{"invalid JSON", 200, "{", "recentchange", "decode EventStreams"},
		{"empty spec", 200, `{}`, "recentchange", "no public streams"},
		{"unknown stream", 200, discoveryFixture, "missing", "unknown public stream"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			discoveryHTTP(t, tt.status, tt.body)
			cmd := NewInfoCmd()
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs([]string{tt.stream})
			if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("want %q error, got %v", tt.want, err)
			}
		})
	}
}
