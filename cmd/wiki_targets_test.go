package cmd

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type targetTransport func(*http.Request) (*http.Response, error)

func (f targetTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestWikimediaWikiHosts(t *testing.T) {
	for _, host := range []string{"en.wikipedia.org", "fr.wiktionary.org", "zh-min-nan.wikipedia.org", "commons.wikimedia.org", "www.wikidata.org", "www.mediawiki.org"} {
		if !isWikimediaWikiHost(host) {
			t.Errorf("wiki not recognised: %s", host)
		}
	}
	for _, host := range []string{"example.org", "en.wikipedia.org.example.org", "gitlab.wikimedia.org", "foo.en.wikipedia.org", "en.wikipedia.org/path", "en.wikipedia.org:443", "/.wikipedia.org"} {
		if isWikimediaWikiHost(host) {
			t.Errorf("non-wiki recognised: %s", host)
		}
	}
}

func TestWikiTargetAnalyticsRequests(t *testing.T) {
	previous := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = previous })
	for _, tc := range []struct {
		name, metric, wantPath string
		args                   []string
	}{
		{"pageviews", "pageviews", "/api/rest_v1/metrics/pageviews/per-article/fr.wikipedia.org/all-access/all-agents/Main%20Page/daily/2025010100/2025010200", []string{"Main Page", "--start", "20250101", "--end", "20250102"}},
		{"project metric", "edits", "/api/rest_v1/metrics/edits/aggregate/fr.wikipedia.org/all-editor-types/all-page-types/daily/20250101/20250103", []string{"--start", "20250101", "--end", "20250102"}},
		{"explicit override", "pageviews", "/api/rest_v1/metrics/pageviews/aggregate/de.wikipedia.org/all-access/all-agents/daily/2025010100/2025010200", []string{"--project", "de.wikipedia.org", "--start", "20250101", "--end", "20250102"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requested := false
			http.DefaultClient = &http.Client{Transport: targetTransport(func(req *http.Request) (*http.Response, error) {
				requested = true
				if req.URL.EscapedPath() != tc.wantPath {
					t.Errorf("request path = %s, want %s", req.URL.EscapedPath(), tc.wantPath)
				}
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"items":[]}`))}, nil
			})}
			root := targetTestRoot()
			root.SetOut(io.Discard)
			root.SetErr(io.Discard)
			args := append([]string{"@fr.wikipedia.org", tc.metric}, tc.args...)
			args = append(args, "--output", "json")
			root.SetArgs(resolveServiceTargetArgs(root, args))
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			if !requested {
				t.Fatal("no analytics request made")
			}
		})
	}
}

func TestWikiTargetEventRequests(t *testing.T) {
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	for _, stream := range []string{"recentchange", "revision-create"} {
		t.Run(stream, func(t *testing.T) {
			http.DefaultTransport = targetTransport(func(req *http.Request) (*http.Response, error) {
				if req.URL.Host != "stream.wikimedia.org" || req.URL.Path != "/v2/stream/"+stream {
					t.Fatalf("unexpected request: %s", req.URL)
				}
				body := "data: {\"meta\":{\"domain\":\"en.wikipedia.org\"},\"id\":1}\n\n" +
					"data: {\"meta\":{\"domain\":\"canary\"},\"id\":2}\n\n" +
					"data: {\"meta\":{\"domain\":\"fr.wikipedia.org\"},\"id\":3}\n\n"
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
			})
			root := targetTestRoot()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			root.SetContext(ctx)
			var output bytes.Buffer
			root.SetOut(&output)
			root.SetErr(io.Discard)
			root.SetArgs(resolveServiceTargetArgs(root, []string{"@fr.wikipedia.org", "events", "tail", "--stream", stream, "--limit", "1"}))
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			if got, want := output.String(), "{\"meta\":{\"domain\":\"fr.wikipedia.org\"},\"id\":3}\n"; got != want {
				t.Errorf("output = %q, want %q", got, want)
			}
		})
	}
}

func TestWikiTargetMenu(t *testing.T) {
	root := targetTestRoot()
	args := resolveServiceTargetArgs(root, []string{"__complete", "@commons.wikimedia.org", ""})
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(io.Discard)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"page", "file", "ext", "pageviews", "top-pages", "edits", "top-edited", "editors", "new-pages", "devices", "events"} {
		if !strings.Contains(output.String(), name+"\t") {
			t.Errorf("menu missing %s: %s", name, output.String())
		}
	}
	if strings.Contains(output.String(), "mediarequests") {
		t.Fatal("global media metrics offered for a site")
	}
	target, _, err := root.Find([]string{"@commons.wikimedia.org"})
	if err != nil {
		t.Fatal(err)
	}
	if target.PersistentFlags().Lookup("wiki") != nil {
		t.Fatal("Action API flags leaked into other providers")
	}
}
