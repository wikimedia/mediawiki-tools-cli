package grafana

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGrafanaReads(t *testing.T) {
	cases := []struct {
		name, method, path, response, want string
		args                               []string
		check                              func(*testing.T, *http.Request)
	}{
		{
			name: "search", method: http.MethodGet, path: "/api/search", response: `[{"uid":"example","title":"Site Health"}]`, want: "example\n",
			args: []string{"dashboard", "list", "Site & Health", "--page", "2", "--limit", "5", "-o", "jq", "--format", ".[].uid"},
			check: func(t *testing.T, r *http.Request) {
				query := r.URL.Query()
				if query.Get("query") != "Site & Health" || query.Get("type") != "dash-db" || query.Get("page") != "2" || query.Get("limit") != "5" {
					t.Errorf("unexpected search parameters: %v", query)
				}
			},
		},
		{
			name: "model", method: http.MethodGet, path: "/api/dashboards/uid/example", response: `{"meta":{"slug":"example"},"dashboard":{"uid":"example"}}`,
			want: "{\"uid\":\"example\"}\n", args: []string{"dashboard", "get", "example"},
		},
		{
			name: "data sources", method: http.MethodGet, path: "/api/datasources", response: `[{"uid":"prom","type":"prometheus"}]`,
			want: "prom\n", args: []string{"datasource", "list", "-o", "jq", "--format", ".[].uid"},
		},
		{
			name: "instant query", method: http.MethodGet, path: "/api/datasources/proxy/uid/prom/api/v1/query", response: `{"status":"success"}`,
			want: "success\n", args: []string{"query", `vector(1) > 0`, "--datasource", "prom", "-o", "jq", "--format", ".status"},
			check: func(t *testing.T, r *http.Request) {
				if q := r.URL.Query(); q.Get("query") != "vector(1) > 0" || q.Has("start") {
					t.Errorf("unexpected instant query: %v", q)
				}
			},
		},
		{
			name: "range query", method: http.MethodGet, path: "/api/datasources/proxy/uid/prom/api/v1/query_range", response: `{"status":"success"}`,
			want: "success\n", args: []string{"query", "vector(1)", "-d", "prom", "--since", "15m", "--step", "30s", "-o", "jq", "--format", ".status"},
			check: func(t *testing.T, r *http.Request) {
				query := r.URL.Query()
				start, startErr := time.Parse(time.RFC3339Nano, query.Get("start"))
				end, endErr := time.Parse(time.RFC3339Nano, query.Get("end"))
				if startErr != nil || endErr != nil || end.Sub(start) != 15*time.Minute || query.Get("step") != "30" {
					t.Errorf("unexpected range query: %v", query)
				}
			},
		},
		{
			name: "native query", method: http.MethodPost, path: "/api/ds/query", response: `{"results":{}}`,
			want: "{\"results\":{}}\n", args: []string{"api", "/api/ds/query", "--input", "-"},
			check: func(t *testing.T, r *http.Request) {
				data, err := io.ReadAll(r.Body)
				if err != nil || string(data) != "{}" || r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("unexpected data query body: %q, %v", data, err)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tc.method || r.URL.Path != tc.path || r.Header.Get("Authorization") != "" || r.Header.Get("User-Agent") == "" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				if tc.check != nil {
					tc.check(t, r)
				}
				_, _ = io.WriteString(w, tc.response)
			}))
			defer server.Close()
			cmd := newCmd(server.URL)
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetIn(strings.NewReader("{}"))
			cmd.SetArgs(tc.args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if out.String() != tc.want {
				t.Fatalf("output = %q, want %q", out.String(), tc.want)
			}
		})
	}
}

func TestGrafanaRejectsInvalidRequests(t *testing.T) {
	for _, args := range [][]string{
		{"api", "https://example.org/api/health"},
		{"api", "//example.org/api/health"},
		{"api", "/api/dashboards/db", "-X", "POST"},
		{"api", "/api/ds/query", "-X", "DELETE"},
		{"dashboard", "get", ".."},
		{"query", "vector(1)"},
		{"query", "vector(1)", "-d", "prom", "--step", "0s"},
		{"dashboard", "list", "-o", "jq", "--format", "("},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			cmd := newCmd("invalid base URL")
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(args)
			if err := cmd.Execute(); err == nil || strings.Contains(err.Error(), "invalid API base URL") {
				t.Fatalf("expected validation error before request, got %v", err)
			}
		})
	}
}
