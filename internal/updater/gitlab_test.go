package updater

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

type releaseTransport func(*http.Request) (*http.Response, error)

func (f releaseTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestUpdateNotificationsUnderstandDatedReleases(t *testing.T) {
	previousTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previousTransport })
	http.DefaultTransport = releaseTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/v4/projects/16/releases" {
			t.Errorf("unexpected request %s", r.URL)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(`[
				{"tag_name":"v2026.10.31.2"},
				{"tag_name":"v0.99.0-rc.1"},
				{"tag_name":"v2026.11.15.0"}
			]`)),
		}, nil
	})
	for _, tc := range []struct {
		current string
		want    bool
	}{
		{"0.34.0", true},
		{"2026.10.31.2", true},
		{"2026.11.15.0", false},
		{"2026.12.01.0", false},
	} {
		got, version := CanUpdateFromGitlab(cli.Version(tc.current), "")
		if got != tc.want || version != "2026.11.15.0" {
			t.Errorf("notification from %s: %t, %q; want %t, 2026.11.15.0", tc.current, got, version, tc.want)
		}
	}
}
