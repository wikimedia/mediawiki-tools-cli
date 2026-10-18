package events

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
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
	if err := tail(context.Background(), "recentchange", "enwiki", "", "", -1, 1, &out); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), "{\"wiki\":\"enwiki\",\"id\":9007199254740993}\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
