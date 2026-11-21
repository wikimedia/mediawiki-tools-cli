package quarry

import (
	"bytes"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestResults(t *testing.T) {
	previous := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = previous })

	for _, tc := range []struct {
		name  string
		args  []string
		paths []string
		body  string
		err   string
	}{
		{
			name:  "latest follows redirect",
			args:  []string{"541"},
			paths: []string{"/query/541/result/latest/0/json", "/run/1534/output/0/json"},
			body:  `{"headers":["count"],"rows":[[88]]}`,
		},
		{
			name:  "exact run and result set",
			args:  []string{"--run", "1534", "--result-set", "1", "--format", "csv"},
			paths: []string{"/run/1534/output/1/csv"},
			body:  "count\n88\n",
		},
		{
			name:  "missing result",
			args:  []string{"--run", "999"},
			paths: []string{"/run/999/output/0/json"},
			err:   "HTTP 404",
		},
		{name: "no ID", err: "provide a query ID"},
		{name: "invalid ID", args: []string{"zero"}, err: "expected a positive integer"},
		{name: "zero run ID", args: []string{"--run", "0"}, err: "positive run ID"},
		{name: "both IDs", args: []string{"541", "--run", "1534"}, err: "in place of the query ID"},
		{name: "invalid format", args: []string{"541", "--format", "html"}, err: "unsupported Quarry format"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var paths []string
			http.DefaultClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				paths = append(paths, r.URL.Path)
				if r.URL.Scheme+"://"+r.URL.Host != BaseURL || r.Method != http.MethodGet || r.UserAgent() != cli.HTTPUserAgent() {
					t.Fatalf("unexpected request %s %s with User-Agent %q", r.Method, r.URL, r.UserAgent())
				}
				resp := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(tc.body))}
				if strings.HasPrefix(r.URL.Path, "/query/") {
					resp.StatusCode = http.StatusFound
					resp.Header.Set("Location", "/run/1534/output/0/json")
				} else if tc.err != "" {
					resp.StatusCode = http.StatusNotFound
					resp.Body = io.NopCloser(strings.NewReader("missing result"))
				}
				return resp, nil
			})}

			var stdout bytes.Buffer
			cmd := NewCmd()
			cmd.SetOut(&stdout)
			cmd.SetArgs(append([]string{"results"}, tc.args...))
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			err := cmd.Execute()
			if tc.err == "" && err != nil || tc.err != "" && (err == nil || !strings.Contains(err.Error(), tc.err)) {
				t.Fatalf("error = %v, want %q", err, tc.err)
			}
			if !slices.Equal(paths, tc.paths) {
				t.Errorf("requests = %v, want %v", paths, tc.paths)
			}
			if stdout.String() != tc.body {
				t.Errorf("stdout = %q, want %q", stdout.String(), tc.body)
			}
		})
	}
}
