package cli

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type requestTransport func(*http.Request) (*http.Response, error)

func (f requestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestHTTPUserAgentPreservesClientAndRequest(t *testing.T) {
	previous := VersionDetails
	VersionDetails = VersionAttributes{Version: "1.2.3"}
	t.Cleanup(func() { VersionDetails = previous })
	for _, explicit := range []string{"", "custom-client/1.0"} {
		t.Run(explicit, func(t *testing.T) {
			want := explicit
			if want == "" {
				want = HTTPUserAgent()
			}
			base := &http.Client{Timeout: 3 * time.Second, Transport: requestTransport(func(r *http.Request) (*http.Response, error) {
				if got := r.UserAgent(); got != want {
					t.Errorf("User-Agent = %q, want %q", got, want)
				}
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("ok"))}, nil
			})}
			client := WithHTTPUserAgent(base)
			if client == base || client.Timeout != base.Timeout {
				t.Fatal("client was not copied with its timeout preserved")
			}
			req, err := http.NewRequest(http.MethodGet, "http://fixture.invalid", nil)
			if err != nil {
				t.Fatal(err)
			}
			if explicit != "" {
				req.Header.Set("User-Agent", explicit)
			} else {
				req.Header = nil
			}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if req.Header.Get("User-Agent") != explicit {
				t.Fatal("transport mutated caller's request")
			}
			if _, wrapped := base.Transport.(userAgentTransport); wrapped {
				t.Fatal("transport mutated caller's client")
			}
		})
	}
}

func TestHTTPUserAgentUsesCurrentBuildVersion(t *testing.T) {
	previous := VersionDetails
	t.Cleanup(func() { VersionDetails = previous })
	for _, tc := range []struct {
		version       Version
		summary, want string
	}{
		{"0.35.0", "v0.34.0-1-g123", "0.35.0"},
		{"2026.11.05.1", "", "2026.11.05.1"},
		{"latest", "v0.35.0-2-g123-dirty", "0.35.0-2-g123-dirty"},
		{"latest", "", "dev"},
		{"", "", "dev"},
	} {
		VersionDetails = VersionAttributes{Version: tc.version, GitSummary: tc.summary}
		want := "a-wikimedia-cli/" + tc.want + " (https://a-wikimedia-cli.toolforge.org/)"
		if got := HTTPUserAgent(); got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	}
}
