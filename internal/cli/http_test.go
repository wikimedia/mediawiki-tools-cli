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
	for _, explicit := range []string{"", "custom-client/1.0"} {
		t.Run(explicit, func(t *testing.T) {
			want := explicit
			if want == "" {
				want = HTTPUserAgent
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
