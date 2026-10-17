package cli

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type eventTransport func(*http.Request) (*http.Response, error)

func (f eventTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestTelemetryConsentAndTestContext(t *testing.T) {
	previousOptions, previousTransport := Opts, http.DefaultTransport
	t.Cleanup(func() { Opts = previousOptions; http.DefaultTransport = previousTransport })
	for _, tc := range []struct {
		name        string
		enabled     bool
		testContext string
	}{
		{"opted in", true, ""}, {"opted out", false, ""}, {"test opted in", true, "1"}, {"test opted out", false, "1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("MWCLI_CONTEXT_TEST", tc.testContext)
			Opts.Telemetry = tc.enabled
			events := NewEvents(filepath.Join(t.TempDir(), ".events"))
			events.AddCommandRunEvent("docker ps", Version("1.2.3"))
			events.AddFeatureUsageEvent("example", Version("1.2.3"))
			allowed := tc.enabled && tc.testContext == ""
			want := 0
			if allowed {
				want = 2
			}
			if got := len(events.RawEvents()); got != want {
				t.Fatalf("got %d stored events, want %d", got, want)
			}
			// Even explicitly submitting previously buffered events must respect consent
			// and the test context, without deleting the user's existing queue.
			seed := "{\"command\":\"docker ps\"}\n"
			if err := os.WriteFile(events.EventFile, []byte(seed), 0o600); err != nil {
				t.Fatal(err)
			}
			calls := 0
			http.DefaultTransport = eventTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.URL.String() != "https://intake-analytics.wikimedia.org/v1/events?hasty=true" {
					t.Errorf("unexpected URL %s", r.URL)
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
			})
			if got := events.EmitEvents(); got != allowed {
				t.Errorf("EmitEvents = %v, want %v", got, allowed)
			}
			if (calls == 1) != allowed {
				t.Errorf("unexpected HTTP request count: %d", calls)
			}
			remaining, err := os.ReadFile(events.EventFile)
			if err != nil {
				t.Fatal(err)
			}
			if allowed && len(remaining) != 0 {
				t.Fatal("successfully emitted queue was not cleared")
			}
			if !allowed && string(remaining) != seed {
				t.Fatal("disabled submission changed the buffered events")
			}
		})
	}
}
