package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

func mockToolcounter(t *testing.T, page, body string, status int) {
	t.Helper()
	previous := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = previous })
	http.DefaultClient = &http.Client{Transport: toolviewsTransport(func(r *http.Request) (*http.Response, error) {
		query := r.URL.Query()
		if r.Method != http.MethodGet || r.URL.Host != "toolcounter.toolforge.org" || r.URL.Path != "/api.php" || query.Get("tool") != "example" || query.Get("page") != page || query.Get("action") != "get" || len(query) != 3 {
			t.Fatalf("unexpected or mutating request: %s %s", r.Method, r.URL)
		}
		if r.UserAgent() != cli.HTTPUserAgent || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Fatalf("unexpected request headers: %v", r.Header)
		}
		return &http.Response{StatusCode: status, Header: http.Header{"Retry-After": []string{"30"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
}

func TestToolcounterOutputs(t *testing.T) {
	for _, format := range []string{"auto", "json", "table", "jq"} {
		t.Run(format, func(t *testing.T) {
			page := "/manual/wiki/?literal=&action=increment"
			stats := toolcounterStats{Tool: "example", Page: page, Views: 21, Sessions: 0}
			body, _ := json.Marshal(stats)
			mockToolcounter(t, page, string(body), 200)
			cmd := newToolcounterCmd()
			var buffer bytes.Buffer
			cmd.SetOut(&buffer)
			args := []string{"example", "--page", page, "--output", format}
			if format == "jq" {
				args = append(args, "--format", ".views")
			}
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if format == "jq" {
				if buffer.String() != "21\n" {
					t.Fatal(buffer.String())
				}
			} else if format == "table" {
				if !strings.Contains(buffer.String(), "Sessions") || !strings.Contains(buffer.String(), page) {
					t.Fatal(buffer.String())
				}
			} else {
				var actual toolcounterStats
				if err := json.Unmarshal(buffer.Bytes(), &actual); err != nil || actual != stats {
					t.Fatalf("output=%s error=%v", buffer.String(), err)
				}
			}
		})
	}
}

func TestToolcounterDefaultPageAndRegistration(t *testing.T) {
	mockToolcounter(t, "index", `{"tool":"example","page":"index","views":0,"sessions":0}`, 200)
	cmd := NewToolsCmd()
	var buffer bytes.Buffer
	cmd.SetOut(&buffer)
	cmd.SetArgs([]string{"toolcounter", "example"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var actual toolcounterStats
	if err := json.Unmarshal(buffer.Bytes(), &actual); err != nil || actual.Page != "index" || actual.Tool != "example" {
		t.Fatalf("output=%s error=%v", buffer.String(), err)
	}
}

func TestToolcounterResponseErrors(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
		status           int
	}{
		{"rate limit", "busy", "Retry-After: 30", 429},
		{"invalid json", "<html>error</html>", "decode Toolcounter response", 200},
		{"api error", `{"error":"unknown action"}`, "Toolcounter: unknown action", 200},
		{"missing counts", `{}`, "missing valid views", 200},
		{"null counts", `{"views":null,"sessions":0}`, "missing valid views", 200},
		{"negative counts", `{"views":-1,"sessions":0}`, "missing valid views", 200},
		{"wrong page", `{"tool":"example","page":"other","views":0,"sessions":0}`, "instead of the requested", 200},
		{"wrong tool", `{"tool":"other","page":"index","views":0,"sessions":0}`, "instead of the requested", 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mockToolcounter(t, "index", tc.body, tc.status)
			_, err := fetchToolcounter(context.Background(), "example", "index")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestToolcounterInvalidArgumentsDoNotRequest(t *testing.T) {
	previous := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = previous })
	http.DefaultClient = &http.Client{Transport: toolviewsTransport(func(r *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected request: %s", r.URL)
		return nil, errors.New("unexpected request")
	})}
	for _, args := range [][]string{
		{},
		{"one", "two"},
		{"bad/name"},
		{"example", "--page", ""},
		{"example", "--page", "\n"},
		{"example", "--timeout", "0s"},
		{"example", "--output", "graph"},
		{"example", "--output", "jq"},
	} {
		cmd := newToolcounterCmd()
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Fatalf("expected error for %v", args)
		}
	}
}

func TestToolcounterTimeout(t *testing.T) {
	previous := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = previous })
	http.DefaultClient = &http.Client{Transport: toolviewsTransport(func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, r.Context().Err()
	})}
	cmd := newToolcounterCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"example", "--timeout", "1ms"})
	if err := cmd.Execute(); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error=%v", err)
	}
}
