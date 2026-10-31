package tools

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/adrg/xdg"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

type toolviewsTransport func(*http.Request) (*http.Response, error)

func (f toolviewsTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func mockToolviews(t *testing.T, path, body string, status int) {
	t.Helper()
	previousCache := xdg.CacheHome
	xdg.CacheHome = t.TempDir()
	t.Cleanup(func() { xdg.CacheHome = previousCache })
	previous := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = previous })
	http.DefaultClient = &http.Client{Transport: toolviewsTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "toolviews.toolforge.org" || r.URL.EscapedPath() != path {
			t.Fatalf("unexpected request: %s", r.URL)
		}
		if r.UserAgent() != cli.HTTPUserAgent || !strings.Contains(r.UserAgent(), "a-wikimedia-cli") {
			t.Fatalf("wrong User-Agent: %q", r.UserAgent())
		}
		if r.Header.Get("Authorization") != "" {
			t.Fatal("public API request carried credentials")
		}
		return &http.Response{StatusCode: status, Header: http.Header{"Retry-After": []string{"60"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
}

func TestToolviewsOutputs(t *testing.T) {
	for _, format := range []string{"json", "table", "csv", "jq", "graph"} {
		t.Run(format, func(t *testing.T) {
			mockToolviews(t, "/api/v1/tool/example/daily/2026-10-01/2026-10-03", `{"results":{"2026-10-03":{"example":7},"2026-10-01":{"example":0}}}`, 200)
			cmd := newToolviewsCmd()
			var out bytes.Buffer
			cmd.SetOut(&out)
			args := []string{"example", "--start", "2026-10-01", "--end", "2026-10-03", "--output", format}
			if format == "jq" {
				args = append(args, "--format", ".[].hits")
			}
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			text := out.String()
			if format == "jq" {
				if text != "0\n7\n" {
					t.Fatal(text)
				}
				return
			}
			if format == "graph" {
				if !strings.Contains(text, "Hits") {
					t.Fatal(text)
				}
				return
			}
			if !strings.Contains(text, "example") || strings.Index(text, "2026-10-01") > strings.Index(text, "2026-10-03") || strings.Contains(text, "2026-10-02") {
				t.Fatalf("lost ordering or invented missing observation: %s", text)
			}
		})
	}
}

func TestToolviewsDayAndRanking(t *testing.T) {
	mockToolviews(t, "/api/v1/day/2026-10-01", `{"results":{"small":0,"large":100}}`, 200)
	cmd := newToolviewsCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--date", "2026-10-01", "-o", "table"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "Hits") || strings.Index(text, "large") > strings.Index(text, "small") {
		t.Fatal(text)
	}
}

func TestToolviewsList(t *testing.T) {
	mockToolviews(t, "/api/v1/tools", `{"results":["zeta","alpha"]}`, 200)
	cmd := newToolviewsCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"list", "-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Index(out.String(), "alpha") > strings.Index(out.String(), "zeta") {
		t.Fatal(out.String())
	}
}

func TestToolviewsErrors(t *testing.T) {
	for _, tc := range []struct {
		body   string
		status int
		want   string
	}{{"busy", 429, "Retry-After: 60"}, {"not json", 200, "decode Toolviews response"}} {
		t.Run(tc.want, func(t *testing.T) {
			mockToolviews(t, "/api/v1/tool/example/day/2026-10-01", tc.body, tc.status)
			cmd := newToolviewsCmd()
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs([]string{"example", "--date", "2026-10-01"})
			if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestToolviewsValidationBeforeRequest(t *testing.T) {
	previous := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = previous })
	http.DefaultClient = &http.Client{Transport: toolviewsTransport(func(*http.Request) (*http.Response, error) { t.Fatal("invalid input made request"); return nil, nil })}
	for _, args := range [][]string{{"--date", "bad"}, {"--date", "2026-10-01", "--start", "2026-10-01"}, {"--start", "2026-10-03", "--end", "2026-10-01"}, {"--graph-width", "2"}, {"--timeout", "0s"}, {"--output", "wrong"}, {"--output", "jq"}, {"has space"}, {"../evil"}, {"--output", "graph"}} {
		cmd := newToolviewsCmd()
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}

func TestToolsAndToolforgeAreSeparate(t *testing.T) {
	root := &cobra.Command{Use: "mw"}
	root.AddCommand(NewToolsCmd(), NewToolforgeCmd())
	for _, path := range [][]string{{"tools", "toolviews"}, {"tools", "views"}, {"toolforge", "shell"}, {"toolforge", "cp"}, {"toolforge", "job"}} {
		found, rest, err := root.Find(path)
		if err != nil || len(rest) > 0 || found == root {
			t.Fatalf("cannot resolve %v: %v %v", path, rest, err)
		}
	}
	if len(NewToolforgeCmd().Aliases) > 0 {
		t.Fatal("Toolforge still aliases tools")
	}
}

func TestToolviewsCompletionUsesLiveNames(t *testing.T) {
	mockToolviews(t, "/api/v1/tools", `{"results":["tool-z","other","tool-a"]}`, 200)
	cmd := newToolviewsCmd()
	cmd.SetContext(t.Context())
	names, directive := cmd.ValidArgsFunction(cmd, nil, "tool-")
	if strings.Join(names, ",") != "tool-a,tool-z" || directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("completion = %v, %v", names, directive)
	}
}

func TestToolviewsCompletionSkipsFurtherArguments(t *testing.T) {
	previous := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = previous })
	http.DefaultClient = &http.Client{Transport: toolviewsTransport(func(*http.Request) (*http.Response, error) {
		t.Fatal("completion fetched names after the tool argument")
		return nil, nil
	})}
	cmd := newToolviewsCmd()
	cmd.SetContext(t.Context())
	names, directive := cmd.ValidArgsFunction(cmd, []string{"tool-a"}, "")
	if len(names) != 0 || directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("completion = %v, %v", names, directive)
	}
}

func TestToolviewsCompletionFailureIsQuiet(t *testing.T) {
	mockToolviews(t, "/api/v1/tools", `unavailable`, 503)
	cmd := newToolviewsCmd()
	cmd.SetContext(t.Context())
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	names, directive := cmd.ValidArgsFunction(cmd, nil, "")
	if len(names) != 0 || directive != cobra.ShellCompDirectiveNoFileComp || stderr.Len() != 0 {
		t.Fatalf("completion = %v, %v, %s", names, directive, stderr.String())
	}
}

func TestToolviewsUniqueFlagRemoved(t *testing.T) {
	cmd := newToolviewsCmd()
	if cmd.Flags().Lookup("unique") != nil {
		t.Fatal("unique flag remains exposed")
	}
}
