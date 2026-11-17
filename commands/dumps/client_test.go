package dumps

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

func testClient(t *testing.T, handler http.HandlerFunc) (*dumpClient, *url.URL) {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	client := newClient(func(*url.URL) error { return nil })
	client.http.Transport = server.Client().Transport
	t.Cleanup(client.http.CloseIdleConnections)
	base, err := url.Parse(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	return client, base
}

func TestDumpPaths(t *testing.T) {
	for input, want := range map[string]string{
		"": "", "current/enwiki/2026-11-01/xml/bzip2/": "other/mediawiki_content_current/enwiki/2026-11-01/xml/bzip2/",
		"wikidata/latest-all.json.bz2": "wikidatawiki/entities/latest-all.json.bz2",
		"commons/":                     "commonswiki/entities/", "sql/enwiki/": "enwiki/", "other/pageviews/": "other/pageviews/",
		"some-new-dataset/file.parquet": "some-new-dataset/file.parquet",
	} {
		got, err := dumpPath(input)
		if err != nil || got != want {
			t.Fatalf("%q = %q, %v; want %q", input, got, err, want)
		}
	}
	for _, input := range []string{"../secret", "a/../secret", "https://example.com/file", "/etc/passwd", "//evil/file", "file?x=y", "file#x", `a\b`, "a%2fb", "a\nb"} {
		if _, err := dumpPath(input); err == nil {
			t.Errorf("accepted path %q", input)
		}
	}
	for _, input := range []string{"http://example.com/", "https://u:p@example.com/", "https://example.com/#fragment", "https://example.com/?a=b", "unknown"} {
		if _, err := mirrorURL(input); err == nil {
			t.Errorf("accepted mirror %q", input)
		}
	}
	for _, m := range mirrors {
		u, err := mirrorURL(m.Name)
		if err != nil || u.String() != m.URL {
			t.Fatalf("mirror %s = %v, %v", m.Name, u, err)
		}
	}
}

func TestListDatasetsAndLandingPages(t *testing.T) {
	client, base := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != cli.HTTPUserAgent() || r.Header.Get("Accept-Encoding") != "identity" {
			t.Error("missing download request headers")
		}
		if r.URL.Path != "/mirror/other/" {
			t.Errorf("path = %s", r.URL.Path)
		}
		fmt.Fprint(w, `<a href="../">parent</a>
<a href="new-format.parquet">file</a><a href="wiki/">wiki</a>
<a href="wiki/">duplicate</a><a href="?C=N">sort</a>
<a href="/mirror/other/pageviews/readme.html">landing page</a>
<a href="/mirror/wikidatawiki/entities/">sibling</a>
<a href="/outside/">outside mirror</a><a href="https://evil.example/file">external</a>
<a href="bad%0Aname">control</a>`)
	})
	base = base.JoinPath("mirror/")
	result, err := client.list(t.Context(), base, "other")
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, entry := range result {
		paths = append(paths, entry.Path)
	}
	want := []string{"other/new-format.parquet", "other/pageviews/readme.html", "other/wiki/", "wikidatawiki/entities/"}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("paths=%q, want %q", paths, want)
	}
}

func TestMetadataFailuresAndPagePaths(t *testing.T) {
	client, base := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/page.html" {
			fmt.Fprint(w, `<a href="directory/">data</a>`)
			return
		}
		if r.URL.Path == "/large" {
			_, _ = io.CopyN(w, strings.NewReader(strings.Repeat("x", (16<<20)+1)), (16<<20)+1)
			return
		}
		http.NotFound(w, r)
	})
	if entries, err := client.list(t.Context(), base, "page.html"); err != nil || len(entries) != 1 || entries[0].Path != "directory/" {
		t.Fatalf("HTML landing page: %v, %v", entries, err)
	}
	for _, p := range []string{"missing", "large"} {
		if _, err := client.metadata(t.Context(), base.JoinPath(p)); err == nil {
			t.Errorf("accepted metadata %s", p)
		}
	}
}

func TestApprovalBoundaries(t *testing.T) {
	opts := &options{}
	root := &cobra.Command{}
	asked := 0
	client := opts.client(root, func(_ *cobra.Command, message string) error {
		asked++
		if !strings.Contains(message, "approve-host") {
			t.Error("approval does not explain explicit host flag")
		}
		return nil
	})
	for _, address := range []string{"https://dumps.wikimedia.org/a", "https://analytics.wikimedia.org/file", "https://dumps.wikimedia.org:443/file"} {
		u, _ := url.Parse(address)
		if err := client.approve(u); err != nil {
			t.Fatal(err)
		}
	}
	if asked != 0 {
		t.Fatal("official host requested third-party approval")
	}
	for _, address := range []string{"https://dumps.wikimedia.your.org/a", "https://dumps.wikimedia.your.org/b", "https://dumps.wikimedia.org:444/a", "https://wikimedia.org.evil.example/a"} {
		u, _ := url.Parse(address)
		if err := client.approve(u); err != nil {
			t.Fatal(err)
		}
	}
	if asked != 3 {
		t.Fatalf("approval calls = %d, want one per external authority", asked)
	}
	denied := opts.client(root, func(*cobra.Command, string) error { return errors.New("denied") })
	target, _ := url.Parse("https://dumps.wikimedia.your.org/file")
	if err := denied.approve(target); err == nil {
		t.Fatal("another invocation inherited approval")
	}
	explicit := (&options{approvedHosts: []string{target.Host}}).client(root, func(*cobra.Command, string) error {
		t.Fatal("explicit host requested approval")
		return nil
	})
	if err := explicit.approve(target); err != nil {
		t.Fatal(err)
	}
}

func TestDeniedRequestsAndRedirectsNeverContactTarget(t *testing.T) {
	hits := 0
	client, base := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "https://external.example/file", http.StatusFound)
		}
	})
	client.approve = func(*url.URL) error { return errors.New("denied") }
	resp, err := client.request(t.Context(), http.MethodGet, base, nil)
	if resp != nil {
		resp.Body.Close()
	}
	if err == nil || hits != 0 {
		t.Fatal("denied request was sent")
	}
	client.approve = func(u *url.URL) error {
		if u.Host == base.Host {
			return nil
		}
		return errors.New("denied redirect")
	}
	resp, err = client.request(t.Context(), http.MethodGet, base.JoinPath("redirect"), nil)
	if resp != nil {
		resp.Body.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "denied redirect") {
		t.Fatalf("redirect error=%v", err)
	}
	if err := client.http.CheckRedirect(&http.Request{URL: &url.URL{Scheme: "http", Host: base.Host}}, nil); err == nil {
		t.Fatal("accepted HTTPS downgrade")
	}
}

func TestSpeedTestBoundedAndApprovalRequired(t *testing.T) {
	const size = 2 << 20
	hits := 0
	client, base := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Header.Get("Range") != "bytes=0-1048575" {
			t.Error("speed request did not ask for a bounded range")
		}
		// A server ignoring Range must not cause an unbounded download.
		fmt.Fprint(w, strings.Repeat("x", size))
	})
	result := client.speed(t.Context(), speedResult{URL: base.JoinPath("file").String()}, 1<<20, time.Second)
	if result.Error != "" || result.Bytes != 1<<20 || result.BytesPerSecond <= 0 {
		t.Fatalf("speed=%+v", result)
	}
	hits = 0
	cmd := newCmd(func(*cobra.Command, string) error { return errors.New("declined") })
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"mirrors", "test", "file", base.String()})
	if err := cmd.Execute(); err == nil || hits != 0 {
		t.Fatal("speed test ran without approval")
	}
}

func TestCatalogCommandsAndInvalidInputs(t *testing.T) {
	for _, args := range [][]string{{"datasets", "--output", "json"}, {"mirrors", "list", "--output", "json"}} {
		cmd := newCmd(func(*cobra.Command, string) error {
			t.Fatal("offline catalog requested network approval")
			return nil
		})
		var output bytes.Buffer
		cmd.SetOut(&output)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil || !strings.HasPrefix(output.String(), "[") {
			t.Fatalf("%q output=%s err=%v", args, output.String(), err)
		}
	}
	for _, args := range [][]string{
		{"list", "../"},
		{"list", "--output", "bad"},
		{"list", "--mirror", "http://example.com"},
		{"download", "file"},
		{"download", "current/", "--file", "test"},
		{"download", "file", "--file", "test", "--sha256", "bad"},
		{"mirrors", "test", "directory/", "--yes"},
		{"mirrors", "test", "file", "unknown", "--yes"},
		{"mirrors", "test", "file", "--sample-mib", "0"},
		{"mirrors", "test", "file", "--sample-mib", "-1"},
		{"mirrors", "test", "file", "--sample-mib", "8796093022208"},
		{"mirrors", "test", "file", "--sample-timeout", "0s"},
		{"mirrors", "test", "file", "--sample-timeout", "-1s"},
	} {
		cmd := newCmd(func(*cobra.Command, string) error { return errors.New("unexpected approval") })
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Errorf("accepted %q", args)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	client, base := testClient(t, func(http.ResponseWriter, *http.Request) {})
	if _, err := client.metadata(ctx, base); err == nil {
		t.Fatal("cancelled metadata request succeeded")
	}
}
