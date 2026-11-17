package dumps

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestMirrorPaths(t *testing.T) {
	for _, tt := range []struct {
		mirror, canonical, mapped string
	}{
		{"umu", "enwiki/20261101/enwiki-20261101-page.sql.gz", "dumps/enwiki/20261101/enwiki-20261101-page.sql.gz"},
		{"umu", "itwiktionary/", "dumps/itwiktionary/"},
		{"umu", "wikidatawiki/entities/latest-all.json.bz2", "other/wikibase/wikidatawiki/latest-all.json.bz2"},
		{"umu", "commonswiki/entities/20261101/file.json.gz", "other/wikibase/commonswiki/20261101/file.json.gz"},
		{"umu", "other/pageviews/2026/2026-11/file.gz", "other/pageviews/2026/2026-11/file.gz"},
		{"umu", "other/mediawiki_content_current/enwiki/2026-11-01/", "other/mediawiki_content_current/enwiki/2026-11-01/"},
		{"scatter", "wikidatawiki/entities/20260501/file.json.gz", "other/wikibase/wikidatawiki/20260501/file.json.gz"},
		{"scatter", "commonswiki/entities/20260501/", "other/wikibase/commonswiki/20260501/"},
		{"scatter", "enwiki/2026-05-01/file.gz", "enwiki/2026-05-01/file.gz"},
		{"your", "enwiki/latest/file.sql.gz", "enwiki/latest/file.sql.gz"},
		{"wikimedia", "wikidatawiki/entities/latest-all.json.bz2", "wikidatawiki/entities/latest-all.json.bz2"},
		{"https://custom.example/dumps/", "enwiki/latest/file.gz", "enwiki/latest/file.gz"},
	} {
		t.Run(tt.mirror+"/"+tt.canonical, func(t *testing.T) {
			base, err := mirrorURL(tt.mirror)
			if err != nil {
				t.Fatal(err)
			}
			target, err := mirrorTarget(base, tt.canonical)
			if err != nil || target.String() != base.JoinPath(tt.mapped).String() {
				t.Fatalf("target=%v, err=%v; want %s", target, err, tt.mapped)
			}
			if got := canonicalMirrorPath(base, tt.mapped); got != tt.canonical {
				t.Fatalf("inverse mapping=%s; want %s", got, tt.canonical)
			}
			explicit, err := mirrorURL(base.String())
			if err != nil {
				t.Fatal(err)
			}
			same, err := mirrorTarget(explicit, tt.canonical)
			if err != nil || same.String() != target.String() {
				t.Fatalf("explicit base target=%v, err=%v; want %s", same, err, target)
			}
			// Paths encountered while browsing the mirror root must not get
			// mapped twice if copied directly from a mirror directory.
			same, err = mirrorTarget(base, tt.mapped)
			if err != nil || same.String() != target.String() {
				t.Fatalf("already-mapped target=%v, err=%v; want %s", same, err, target)
			}
		})
	}
	for _, tt := range []struct{ mirror, path string }{
		{"umu", "enwiki/latest/"},
		{"umu", "enwiki/latest/file.sql.gz"},
		{"umu", "dumps/enwiki/latest/file.sql.gz"},
		{"scatter", "enwiki/latest/file.sql.gz"},
		{"scatter", "wikidatawiki/entities/latest-all.json.bz2"},
		{"scatter", "other/wikibase/wikidatawiki/latest-all.json.bz2"},
	} {
		base, err := mirrorURL(tt.mirror)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := mirrorTarget(base, tt.path); err == nil || !strings.Contains(err.Error(), "dated path") {
			t.Errorf("%s %s: expected dated-path guidance, got %v", tt.mirror, tt.path, err)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestMirrorListingsReturnReusablePaths(t *testing.T) {
	for _, tt := range []struct {
		mirror, directory, request, link, canonical string
	}{
		{"umu", "enwiki", "dumps/enwiki/", "20261101/", "enwiki/20261101/"},
		{"umu", "enwiki/20261101", "dumps/enwiki/20261101/", "enwiki-20261101-page.sql.gz", "enwiki/20261101/enwiki-20261101-page.sql.gz"},
		{"umu", "wikidatawiki/entities", "other/wikibase/wikidatawiki/", "latest-all.json.bz2", "wikidatawiki/entities/latest-all.json.bz2"},
		{"scatter", "wikidatawiki/entities/20260501", "other/wikibase/wikidatawiki/20260501/", "file.json.gz", "wikidatawiki/entities/20260501/file.json.gz"},
	} {
		t.Run(tt.mirror+"/"+tt.directory, func(t *testing.T) {
			base, err := mirrorURL(tt.mirror)
			if err != nil {
				t.Fatal(err)
			}
			client := newClient(func(u *url.URL) error {
				if u.Host != base.Host {
					t.Errorf("approval host=%s; want %s", u.Host, base.Host)
				}
				return nil
			})
			client.http.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if got := req.URL.String(); got != base.JoinPath(tt.request).String() {
					t.Errorf("list request=%s; want %s", got, tt.request)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(fmt.Sprintf(`<a href="%s">entry</a>`, tt.link))),
				}, nil
			})
			entries, err := client.list(t.Context(), base, tt.directory)
			if err != nil || len(entries) != 1 {
				t.Fatalf("entries=%v, err=%v", entries, err)
			}
			if entries[0].Path != tt.canonical {
				t.Fatalf("path=%s; want %s", entries[0].Path, tt.canonical)
			}
			target, err := mirrorTarget(base, entries[0].Path)
			if err != nil || target.String() != entries[0].URL {
				t.Fatalf("listed path maps to %v, %v; want %s", target, err, entries[0].URL)
			}
		})
	}
}

func TestSpeedSampleConfiguredByteLimit(t *testing.T) {
	for _, size := range []int64{2 << 20, 32 << 20} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			client, base := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				wantRange := fmt.Sprintf("bytes=0-%d", size-1)
				if got := r.Header.Get("Range"); got != wantRange {
					t.Errorf("range=%q; want %q", got, wantRange)
				}
				// Ignore Range and offer more than the sample budget.
				_, _ = io.CopyN(w, strings.NewReader(strings.Repeat("x", int(size)+1024)), size+1024)
			})
			result := client.speed(t.Context(), speedResult{URL: base.String()}, size, 5*time.Second)
			if result.Error != "" || result.Bytes != size || result.BytesPerSecond <= 0 {
				t.Fatalf("speed=%+v", result)
			}
		})
	}
}

func TestSpeedSampleTimeLimit(t *testing.T) {
	for _, tt := range []struct {
		name       string
		data       bool
		parentEnds bool
		wantError  bool
	}{
		{name: "sample deadline with data", data: true},
		{name: "sample deadline without data", wantError: true},
		{name: "parent deadline with data", data: true, parentEnds: true, wantError: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client, base := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				if tt.data {
					fmt.Fprint(w, "data")
					w.(http.Flusher).Flush()
				}
				<-r.Context().Done()
			})
			parent := t.Context()
			timeout := 300 * time.Millisecond
			if tt.parentEnds {
				ctx, cancel := context.WithTimeout(parent, timeout)
				defer cancel()
				parent = ctx
				timeout = 5 * time.Second
			}
			start := time.Now()
			result := client.speed(parent, speedResult{URL: base.String()}, 32<<20, timeout)
			if (result.Error != "") != tt.wantError {
				t.Fatalf("speed=%+v", result)
			}
			if tt.data && result.Bytes != 4 {
				t.Fatalf("speed=%+v; expected received bytes", result)
			}
			if elapsed := time.Since(start); elapsed > 3*time.Second {
				t.Fatalf("sample exceeded time budget: %s", elapsed)
			}
		})
	}
}

func TestSpeedApprovalShowsConfiguredLimitsAndMappedURL(t *testing.T) {
	for _, tt := range []struct {
		args []string
		want string
	}{
		{want: "32 MiB or 10s each, whichever comes first"},
		{args: []string{"--sample-mib", "64", "--sample-timeout", "15s"}, want: "64 MiB or 15s each, whichever comes first"},
	} {
		stop := errors.New("stop before network")
		asked := false
		cmd := newCmd(func(_ *cobra.Command, message string) error {
			asked = true
			if !strings.Contains(message, tt.want) {
				t.Errorf("prompt=%q; want %q", message, tt.want)
			}
			return stop
		})
		var output bytes.Buffer
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		cmd.SetArgs(append([]string{"mirrors", "test", "enwiki/20261101/file.sql.gz", "umu"}, tt.args...))
		if err := cmd.Execute(); !errors.Is(err, stop) || !asked {
			t.Fatalf("approval asked=%t, err=%v", asked, err)
		}
		if !strings.Contains(output.String(), "https://ftp.acc.umu.se/mirror/wikimedia.org/dumps/enwiki/20261101/file.sql.gz") {
			t.Fatalf("missing mapped target in preview: %s", output.String())
		}
	}
}

func TestSpeedRejectsInvalidLimitsBeforeApproval(t *testing.T) {
	for _, args := range [][]string{
		{"--sample-mib", "0"},
		{"--sample-mib", "-1"},
		{"--sample-mib", "8796093022208"},
		{"--sample-timeout", "0s"},
		{"--sample-timeout", "-1s"},
	} {
		cmd := newCmd(func(*cobra.Command, string) error {
			t.Error("invalid sample limits reached approval")
			return errors.New("unexpected approval")
		})
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(append([]string{"mirrors", "test", "file"}, args...))
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), args[0]) {
			t.Errorf("args=%v, err=%v", args, err)
		}
	}
}

func TestUnavailableMirrorAliasesFailBeforeApproval(t *testing.T) {
	for _, args := range [][]string{
		{"list", "enwiki/latest", "--mirror", "umu"},
		{"download", "enwiki/latest/file.sql.gz", "--mirror", "umu", "--file", "unused"},
		{"mirrors", "test", "enwiki/latest/file.sql.gz", "umu"},
	} {
		cmd := newCmd(func(*cobra.Command, string) error {
			t.Error("unavailable alias reached network approval")
			return errors.New("unexpected approval")
		})
		var output bytes.Buffer
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		cmd.SetArgs(args)
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error()+output.String(), "dated path") {
			t.Errorf("args=%v, output=%s, err=%v", args, output.String(), err)
		}
	}
}
