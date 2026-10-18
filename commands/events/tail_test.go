package events

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type eventTransport func(*http.Request) (*http.Response, error)

func (f eventTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestTailFormatsMatchingEventsAsJSONLines(t *testing.T) {
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	http.DefaultTransport = eventTransport(func(*http.Request) (*http.Response, error) {
		body := "data: {\"wiki\":\"dewiki\"}\n\n" +
			"data: {\n" +
			"data: \"wiki\": \"enwiki\", \"id\": 9007199254740993\n" +
			"data: }\n\n"
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	var out bytes.Buffer
	if err := tail(context.Background(), tailOptions{stream: "recentchange", wiki: "enwiki", namespace: -1, limit: 1}, &out); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), "{\"wiki\":\"enwiki\",\"id\":9007199254740993}\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestTailTimeWindow(t *testing.T) {
	mockEventHTTP(t, func(req *http.Request) (*http.Response, error) {
		if got := req.URL.Query().Get("since"); got != "2026-10-18T12:00:00Z" {
			t.Fatalf("unexpected since parameter: %q", got)
		}
		body := testSSEEvent(`{"meta":{"domain":"canary","dt":"2026-10-19T12:00:00Z"}}`, "2026-10-18T11:59:59Z")
		for _, event := range []struct{ data, stamp string }{
			{`{"wiki":"enwiki","meta":{"dt":"2026-10-18T11:59:59Z"}}`, "2026-10-18T11:59:59Z"},
			{`{"wiki":"enwiki","meta":{"dt":"2026-10-18T12:00:00Z"}}`, "2026-10-18T12:00:00Z"},
			{`{"wiki":"enwiki","meta":{"dt":"2026-10-18T12:00:59Z"}}`, "2026-10-18T12:00:59Z"},
			// The end bound must stop even when this event does not match --wiki.
			{`{"wiki":"dewiki","meta":{"dt":"2026-10-18T12:01:00Z"}}`, "2026-10-18T12:01:00Z"},
			{`{"wiki":"enwiki","meta":{"dt":"2026-10-18T12:00:30Z"}}`, "2026-10-18T12:01:01Z"},
		} {
			body += testSSEEvent(event.data, event.stamp)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	cmd := NewTailCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--wiki", "enwiki", "--since", "2026-10-18T12:00:00Z", "--until", "2026-10-18T12:01:00Z"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	want := "{\"wiki\":\"enwiki\",\"meta\":{\"dt\":\"2026-10-18T12:00:00Z\"}}\n" +
		"{\"wiki\":\"enwiki\",\"meta\":{\"dt\":\"2026-10-18T12:00:59Z\"}}\n"
	if out.String() != want {
		t.Fatalf("unexpected time window output: %q", out.String())
	}
}

func TestTailInvalidTimeBounds(t *testing.T) {
	mockEventHTTP(t, func(*http.Request) (*http.Response, error) {
		t.Fatal("invalid flags must fail before making an HTTP request")
		return nil, nil
	})
	for _, args := range [][]string{
		{"--since", "yesterday"},
		{"--until", "2026-10-18"},
		{"--since", "2026-10-18T12:00:00Z", "--until", "2026-10-18T12:00:00Z"},
		{"--since", "2026-10-18T12:00:00Z", "--until", "2026-10-18T11:00:00Z"},
	} {
		cmd := NewTailCmd()
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Fatalf("expected error for %v", args)
		}
	}
}

func TestEventTime(t *testing.T) {
	for _, tt := range []struct{ data, want string }{
		{`{"meta":{"dt":"2026-10-18T12:00:00Z"},"dt":"2026-10-18T11:00:00Z"}`, "2026-10-18T12:00:00Z"},
		{`{"dt":"2026-10-18T12:00:00Z"}`, "2026-10-18T12:00:00Z"},
		{`{"timestamp":1}`, "1970-01-01T00:00:01Z"},
		{`{}`, ""},
		{`{"meta":{"dt":"broken"}}`, ""},
	} {
		var event streamEvent
		if err := json.Unmarshal([]byte(tt.data), &event); err != nil {
			t.Fatal(err)
		}
		stamp, err := event.eventTime()
		if tt.want == "" {
			if err == nil {
				t.Fatalf("expected timestamp error for %s", tt.data)
			}
		} else if err != nil || stamp.Format(time.RFC3339) != tt.want {
			t.Fatalf("%s: got %v, %v; want %s", tt.data, stamp, err, tt.want)
		}
	}
}

func TestTailOtherStreamWithTimeBounds(t *testing.T) {
	mockEventHTTP(t, func(*http.Request) (*http.Response, error) {
		// Non-recentchange streams may use incompatible types for recentchange fields.
		body := testSSEEvent(`{"user":{"name":"Example"},"dt":"2026-10-18T12:00:00Z"}`, "2026-10-18T12:00:00Z") +
			testSSEEvent(`{"dt":"2026-10-18T12:01:00Z"}`, "2026-10-18T12:01:00Z")
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	cmd := NewTailCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--stream", "page-create", "--until", "2026-10-18T12:01:00Z"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), "{\"user\":{\"name\":\"Example\"},\"dt\":\"2026-10-18T12:00:00Z\"}\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestTailResumeKeepsTimeBounds(t *testing.T) {
	requests := 0
	mockEventHTTP(t, func(req *http.Request) (*http.Response, error) {
		requests++
		if req.URL.Query().Get("since") != "2026-10-18T12:00:00Z" {
			t.Fatal("missing since parameter")
		}
		body := testSSEEvent(`{"meta":{"dt":"2026-10-18T12:00:00Z"}}`, "2026-10-18T12:00:00Z")
		if requests > 1 {
			if req.Header.Get("Last-Event-ID") != strings.SplitN(body, "\n", 2)[0][4:] {
				t.Fatal("missing resume checkpoint")
			}
			body = testSSEEvent(`{"meta":{"dt":"2026-10-18T12:01:00Z"}}`, "2026-10-18T12:01:00Z")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := NewTailCmd()
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	cmd.SetArgs([]string{"--since", "2026-10-18T12:00:00Z", "--until", "2026-10-18T12:01:00Z"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("expected reconnect, got %d requests", requests)
	}
}

func testSSEEvent(data, stamp string) string {
	parsed, err := time.Parse(time.RFC3339Nano, stamp)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("id: [{\"topic\":\"eqiad.mediawiki.recentchange\",\"partition\":0,\"timestamp\":%d}]\ndata: %s\n\n", parsed.UnixMilli(), data)
}

func TestTailUntilWaitsForAllPartitions(t *testing.T) {
	mockEventHTTP(t, func(*http.Request) (*http.Response, error) {
		// The idle codfw partition jumps past --until before the busy eqiad
		// partition delivers its first historical event, as on the live service.
		body := `id: [{"topic":"eqiad.mediawiki.recentchange","partition":0,"offset":123},{"topic":"codfw.mediawiki.recentchange","partition":0,"timestamp":1792316046950}]
data: {"wiki":"jawiki","meta":{"dt":"2026-10-18T09:34:06.949Z"}}

id: [{"topic":"eqiad.mediawiki.recentchange","partition":0,"timestamp":1792314000001},{"topic":"codfw.mediawiki.recentchange","partition":0,"timestamp":1792316046950}]
data: {"wiki":"enwiki","meta":{"dt":"2026-10-18T09:00:00Z"}}

id: [{"topic":"eqiad.mediawiki.recentchange","partition":0,"timestamp":1792315800001},{"topic":"codfw.mediawiki.recentchange","partition":0,"timestamp":1792316046950}]
data: {"wiki":"enwiki","meta":{"dt":"2026-10-18T09:30:00Z"}}

`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	cmd := NewTailCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--wiki", "enwiki", "--since", "2026-10-18T09:00:00Z", "--until", "2026-10-18T09:30:00Z"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), "{\"wiki\":\"enwiki\",\"meta\":{\"dt\":\"2026-10-18T09:00:00Z\"}}\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestPartitionsPastUntil(t *testing.T) {
	until := time.UnixMilli(1000)
	for _, tt := range []struct {
		id              string
		complete, fails bool
	}{
		{`[{"timestamp":999},{"timestamp":1001}]`, false, false},
		{`[{"offset":123},{"timestamp":1001}]`, false, false},
		{`[{"timestamp":1000},{"timestamp":1001}]`, true, false},
		{`[]`, false, true},
		{`not JSON`, false, true},
	} {
		got, err := partitionsPastUntil(tt.id, until)
		if got != tt.complete || (err != nil) != tt.fails {
			t.Fatalf("%s: got %v, %v", tt.id, got, err)
		}
	}
}

func TestTailEmitsFinalEventBeforeCompletedCheckpoint(t *testing.T) {
	mockEventHTTP(t, func(*http.Request) (*http.Response, error) {
		// The checkpoint is the next position; the current event still belongs
		// inside the window even when that position reaches the end timestamp.
		body := testSSEEvent(`{"meta":{"dt":"2026-10-18T12:00:59.999Z"}}`, "2026-10-18T12:01:00Z")
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	cmd := NewTailCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--until", "2026-10-18T12:01:00Z"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), "{\"meta\":{\"dt\":\"2026-10-18T12:00:59.999Z\"}}\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
