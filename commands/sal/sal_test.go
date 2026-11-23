package sal

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

// Matches the semantic markup emitted by SAL's data/templates/sal.html and
// log.html, including adjacent tbody groups and linkified message text.
const searchHTML = `<html><div class="hits">1-2 of 2 results</div><table class="sal">
<tbody><tr><th><a class="day" href="/production?d=2026-01-22">2026-01-22</a></th></tr>
<tr><td class="time"><a href="/log/abc">13:10</a></td><td class="nick">&lt;alice@deploy1003&gt;</td><td class="message">Finished <a href="https://phabricator.wikimedia.org/T123">T123</a> &amp; sync</td><td class="project">[production]</td></tr>
<tbody><tr><th><a class="day" href="/releng?d=2026-01-21">2026-01-21</a></th></tr>
<tr><td class="time"><a href="/log/def">01:09</a></td><td class="nick">&lt;bob&gt;</td><td class="message">Rebuilt image</td><td class="project">[releng]</td></tr></table></html>`

const atomXML = `<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom"><entry><title>Repool &#039;db1222&#039;</title><id>https://sal.toolforge.org/log/abc</id><published>2026-01-22T13:10:20Z</published><author><name>alice</name></author></entry><entry><title>Another entry</title><published>2026-01-22T13:00:00Z</published></entry></feed>`

func TestParseSearch(t *testing.T) {
	entries, err := parseSearch(searchHTML)
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{
		{Timestamp: "2026-01-22T13:10:00Z", User: "alice@deploy1003", Project: "production", Message: "Finished T123 & sync", URL: baseURL + "/log/abc"},
		{Timestamp: "2026-01-21T01:09:00Z", User: "bob", Project: "releng", Message: "Rebuilt image", URL: baseURL + "/log/def"},
	}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("got %#v, want %#v", entries, want)
	}
	for _, body := range []string{"<html>service unavailable</html>", strings.ReplaceAll(searchHTML, `class="time"`, `class="changed"`), strings.ReplaceAll(searchHTML, "2026-01-22", "not-a-date")} {
		if _, err := parseSearch(body); err == nil {
			t.Errorf("expected malformed response error for %s", body)
		}
	}
	entries, err = parseSearch(`<div class="hits">0 results (0ms)</div><table class="sal"></table>`)
	if err != nil || len(entries) != 0 {
		t.Fatalf("empty search: %#v, %v", entries, err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCommands(t *testing.T) {
	previous := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = previous })
	for _, tc := range []struct {
		name                   string
		args                   []string
		path, query, body, err string
		status                 int
		count                  int
	}{
		{name: "recent atom", args: []string{"recent", "--limit", "1"}, path: "/atom/production", body: atomXML, count: 1},
		{name: "all atom", args: []string{"recent", "--all"}, path: "/atom/__all__", body: atomXML, count: 2},
		{name: "search pagination", args: []string{"search", "scap", "sync", "--all", "--page", "2", "--limit", "2", "--before", "2026-01-31"}, path: "/__all__", query: "d=2026-01-31&i=2&p=1&q=scap+sync", body: searchHTML, count: 2},
		{name: "project", args: []string{"search", "T123", "--project", "deployment-prep"}, path: "/deployment-prep", query: "i=20&p=0&q=T123", body: searchHTML, count: 2},
		{name: "HTTP error", args: []string{"recent"}, path: "/atom/production", status: 503, err: "HTTP 503"},
		{name: "invalid atom", args: []string{"recent"}, path: "/atom/production", body: "<html></html>", err: "decode SAL Atom"},
		{name: "invalid project", args: []string{"recent", "--project", "../about"}, err: "invalid SAL project"},
		{name: "conflicting projects", args: []string{"recent", "--all", "--project", "production"}, err: "either --all or --project"},
		{name: "invalid date", args: []string{"search", "scap", "--before", "2026-02-30"}, err: "before must be"},
		{name: "invalid page", args: []string{"search", "scap", "--page", "0"}, err: "page must"},
		{name: "recent limit", args: []string{"recent", "--limit", "51"}, err: "between 1 and 50"},
		{name: "search limit", args: []string{"search", "scap", "--limit", "201"}, err: "between 1 and 200"},
		{name: "empty query", args: []string{"search", " "}, err: "must not be empty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			http.DefaultClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				requests++
				if r.URL.Host != "sal.toolforge.org" || r.URL.Path != tc.path || r.URL.RawQuery != tc.query || r.UserAgent() != cli.HTTPUserAgent() {
					t.Fatalf("unexpected request %s UA %q", r.URL, r.UserAgent())
				}
				status := tc.status
				if status == 0 {
					status = 200
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(tc.body)), Header: make(http.Header)}, nil
			})}
			cmd := NewCmd()
			var stdout bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetArgs(append(tc.args, "--output", "json"))
			cmd.SilenceErrors, cmd.SilenceUsage = true, true
			err := cmd.Execute()
			if tc.err != "" {
				if err == nil || !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("got %v, want %q", err, tc.err)
				}
				if tc.path == "" && requests != 0 {
					t.Fatal("validation performed request")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var entries []Entry
			if err := json.Unmarshal(stdout.Bytes(), &entries); err != nil {
				t.Fatal(err)
			}
			if len(entries) != tc.count {
				t.Fatalf("got %d entries: %s", len(entries), stdout.String())
			}
			if tc.name == "all atom" && entries[0].Project != "" {
				t.Fatal("all-project feed must not guess project")
			}
			if tc.name == "recent atom" && entries[0].Message != "Repool 'db1222'" {
				t.Fatalf("unexpected Atom message %q", entries[0].Message)
			}
		})
	}
}
