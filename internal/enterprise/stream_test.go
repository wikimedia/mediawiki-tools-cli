package enterprise

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStreamCheckpointRecoveryAndBinding(t *testing.T) {
	file := filepath.Join(t.TempDir(), "checkpoint.json")
	requests := 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("Accept") != "application/x-ndjson" {
			t.Error("not requesting NDJSON")
		}
		var query struct {
			Since  map[string]string `json:"since_per_partition"`
			Parts  []int             `json:"parts"`
			Fields []string          `json:"fields"`
		}
		if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
			t.Error(err)
		}
		if len(query.Parts) != 1 || query.Parts[0] != 0 {
			t.Error("wrong partitions")
		}
		if len(query.Fields) != 3 {
			t.Error("resume fields missing")
		}
		if requests == 2 && query.Since["0"] != "2026-11-04T12:00:01Z" {
			t.Error("did not resume at last acknowledged event", query.Since)
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprintf(w, "{\"name\":\"Squirrel\",\"event\":{\"partition\":0,\"date_published\":\"2026-11-04T12:00:0%dZ\"}}\n", requests)
	})
	opts := StreamOptions{Resource: "articles", Parts: []int{0}, Query: map[string]any{"fields": []string{"name"}}, Since: time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC), Limit: 1, Checkpoint: file}
	var out bytes.Buffer
	if err := c.Stream(t.Context(), opts, &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	opts.Since = time.Time{}
	if err := c.Stream(t.Context(), opts, &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), "Squirrel") != 2 {
		t.Fatal("events lost")
	}
	opts.Resource = "wikidata"
	if err := c.Stream(t.Context(), opts, &out, io.Discard); err == nil {
		t.Fatal("reused checkpoint for different stream")
	}
	if requests != 2 {
		t.Fatal("invalid checkpoint made a request")
	}
}

func TestStreamReconnectAndCancellation(t *testing.T) {
	requests := 0
	c := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		requests++
		if requests == 1 {
			w.WriteHeader(503)
			return
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprintln(w, `{"event":{"partition":0,"date_published":"2026-11-04T12:00:00Z"}}`)
	})
	if err := c.Stream(t.Context(), StreamOptions{Resource: "articles", Limit: 1}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatal("did not reconnect")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := c.Stream(ctx, StreamOptions{Resource: "articles"}, io.Discard, io.Discard); !errors.Is(err, context.Canceled) {
		t.Fatal("lost cancellation", err)
	}
}

func TestStreamRejectsInvalidRecordsAndOutputFailures(t *testing.T) {
	for _, record := range []string{`not-json`, `{"event":{}}`, `{"event":{"partition":10,"date_published":"2026-11-04T12:00:00Z"}}`} {
		c := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/x-ndjson")
			fmt.Fprintln(w, record)
		})
		if err := c.Stream(t.Context(), StreamOptions{Resource: "articles", Limit: 1}, io.Discard, io.Discard); err == nil {
			t.Fatal("accepted malformed event")
		}
	}
	c := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprintln(w, `{"event":{"partition":0,"date_published":"2026-11-04T12:00:00Z"}}`)
	})
	if err := c.Stream(t.Context(), StreamOptions{Resource: "articles", Limit: 1}, failingWriter{}, io.Discard); err == nil {
		t.Fatal("ignored output failure")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("output unavailable") }

func TestStreamReconnectsAfterTruncatedRecord(t *testing.T) {
	requests := 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/x-ndjson")
		if requests == 1 {
			fmt.Fprintln(w, `{"name":"complete","event":{"partition":0,"date_published":"2026-11-04T12:00:01Z"}}`)
			fmt.Fprint(w, `{"name":"interrup`)
			return
		}
		var query struct {
			Since map[string]string `json:"since_per_partition"`
		}
		if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
			t.Error(err)
		}
		if query.Since["0"] != "2026-11-04T12:00:01Z" {
			t.Error("checkpoint advanced beyond the completed event", query.Since)
		}
		fmt.Fprintln(w, `{"name":"recovered","event":{"partition":0,"date_published":"2026-11-04T12:00:02Z"}}`)
	})
	var out bytes.Buffer
	err := c.Stream(t.Context(), StreamOptions{Resource: "articles", Parts: []int{0}, Limit: 2, Since: time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)}, &out, io.Discard)
	if err != nil || requests != 2 || strings.Contains(out.String(), "interrup") || !strings.Contains(out.String(), "recovered") {
		t.Fatal("truncated record broke recovery", err, requests, out.String())
	}
}
