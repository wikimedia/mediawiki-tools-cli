package quip

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

const testQuotes = `<div class="quote"><p>&lt;user&gt; Hello<br>world!</p><ul class="nav"><li><a href="/quip/abc-123">[link]</a></li></ul></div>`

func TestQuipCommands(t *testing.T) {
	for _, tc := range []struct {
		name              string
		args              []string
		path, query, want string
	}{
		{"random", []string{"--no-fun", "--link"}, "/random", "", "<user> Hello\nworld!\n%s/quip/abc-123\n"},
		{"specific", []string{"abc-123", "--no-fun"}, "/quip/abc-123", "", "<user> Hello\nworld!\n"},
		{"top", []string{"top", "--no-fun"}, "/top", "", "<user> Hello\nworld!\n"},
		{"search", []string{"search", "unit tests & bugs", "--page", "2", "--no-fun"}, "/search", "unit tests & bugs", "<user> Hello\nworld!\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tc.path {
					t.Errorf("path = %q", r.URL.Path)
				}
				if r.UserAgent() != cli.HTTPUserAgent {
					t.Errorf("User-Agent = %q", r.UserAgent())
				}
				if tc.query != "" && (r.URL.Query().Get("q") != tc.query || r.URL.Query().Get("p") != "1") {
					t.Errorf("query = %s", r.URL.RawQuery)
				}
				fmt.Fprint(w, testQuotes)
			}))
			defer server.Close()
			cmd := newQuipCmd(server.Client(), server.URL)
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetArgs(tc.args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			want := tc.want
			if strings.Contains(want, "%s") {
				want = fmt.Sprintf(want, server.URL)
			}
			if out.String() != want {
				t.Fatalf("output = %q, want %q", out.String(), want)
			}
		})
	}
}

func TestQuipJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, testQuotes+testQuotes) }))
	defer server.Close()
	cmd := newQuipCmd(server.Client(), server.URL)
	cmd.SetArgs([]string{"top", "--output", "json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got []quote
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "abc-123" || got[0].Text != "<user> Hello\nworld!" || got[0].URL != server.URL+"/quip/abc-123" {
		t.Fatalf("quotes = %#v", got)
	}
}

func TestQuipErrorsAndEmptySearch(t *testing.T) {
	for _, tc := range []struct {
		name       string
		args       []string
		status     int
		body, want string
	}{
		{"service error", nil, 503, "unavailable", "quip service returned HTTP 503"},
		{"invalid page", []string{"search", "test", "--page", "0"}, 200, testQuotes, "--page must be at least 1"},
		{"invalid ID", []string{"../secret"}, 200, testQuotes, "invalid quip ID"},
		{"invalid output", []string{"--output", "xml"}, 200, testQuotes, "invalid --output"},
		{"missing quip", nil, 200, "<html>error page</html>", "quip service returned no quips"},
		{"missing JSON quip", []string{"--output", "json"}, 200, "<html>error page</html>", "quip service returned no quips"},
		{"empty search", []string{"search", "missing", "--no-fun"}, 200, "<div class=results></div>", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); fmt.Fprint(w, tc.body) }))
			defer server.Close()
			cmd := newQuipCmd(server.Client(), server.URL)
			cmd.SetArgs(tc.args)
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			err := cmd.Execute()
			if tc.want != "" {
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("error = %v", err)
				}
			} else if err != nil || out.String() != "No quips found.\n" {
				t.Fatalf("error = %v, output = %q", err, out.String())
			}
		})
	}
}

func TestQuipCancellation(t *testing.T) {
	cmd := newQuipCmd(http.DefaultClient, "https://bash.toolforge.org")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--no-fun"})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("error = %v", err)
	}
}

func TestQuipVisibility(t *testing.T) {
	if NewQuipCmd().Hidden {
		t.Fatal("tools command should be visible")
	}
	if !NewLegacyCmd().Hidden {
		t.Fatal("legacy command should be hidden")
	}
}
