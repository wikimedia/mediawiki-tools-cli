package grafana

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

const panelFixture = `{
  "dashboard": {
    "uid": "example",
    "panels": [
      {"type":"row","id":99,"title":"Row","panels":[
        {"id":7,"title":"Requests","datasource":{"uid":"prom","type":"prometheus"},"targets":[
          {"refId":"A","expr":"rate(requests_total{site=\"$site\"}[$__rate_interval])","range":true},
          {"refId":"B","expr":"sum(up)","instant":true},
          {"refId":"C","hide":true,"datasource":{"uid":"graphite","type":"graphite"}}
        ]}
      ]}
    ],
    "templating":{"list":[{"name":"site","current":{"value":"eqiad"}}]}
  }
}`

func TestInspectDashboardPanels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/search":
			if r.URL.Query().Get("query") != "Requests dashboard" {
				t.Errorf("unexpected search arguments %v", r.URL.Query())
			}
			_, _ = io.WriteString(w, `[{"uid":"other","title":"Requests dashboard archive"},{"uid":"example","title":"Requests Dashboard"}]`)
		case "/api/dashboards/uid/example":
			_, _ = io.WriteString(w, panelFixture)
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	for _, tc := range []struct {
		args []string
		want []string
	}{
		{[]string{"dashboard", "panels", "example"}, []string{"ID", "7", "Requests", "prom"}},
		{[]string{"dashboard", "panels", "example", "--panel", "7"}, []string{"Requests (panel 7)", "Query A", "$__rate_interval", "Query C (hidden)"}},
		{[]string{"dashboard", "panels", "Requests dashboard", "--panel", "requests"}, []string{"Requests (panel 7)"}},
		{[]string{"dashboard", "panels", "example", "--panel", "7", "-o", "jq", "--format", ".queries[0].datasource.uid"}, []string{"prom\n"}},
	} {
		cmd := newCmd(server.URL)
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs(tc.args)
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		for _, want := range tc.want {
			if !strings.Contains(out.String(), want) {
				t.Fatalf("output %q does not contain %q", out.String(), want)
			}
		}
		if strings.Contains(out.String(), "Row") {
			t.Fatalf("layout row appeared in panel output: %q", out.String())
		}
	}
}

func TestDashboardPanelWebOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/dashboards/uid/example" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = io.WriteString(w, panelFixture)
	}))
	defer server.Close()
	var opened string
	cmd := newDashboardQueryCmd(server.URL, func(link string) error {
		opened = link
		return nil
	})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"example", "Requests", "--since", "15m", "--var", "site=eqiad & codfw", "--output", "web"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	link, err := url.Parse(opened)
	if err != nil || link.Path != "/d/example" || !strings.HasPrefix(opened, server.URL+"/") {
		t.Fatalf("unexpected browser URL %q: %v", opened, err)
	}
	params := link.Query()
	from, fromErr := strconv.ParseInt(params.Get("from"), 10, 64)
	to, toErr := strconv.ParseInt(params.Get("to"), 10, 64)
	if params.Get("viewPanel") != "7" || params.Get("var-site") != "eqiad & codfw" || fromErr != nil || toErr != nil || to-from != (15*time.Minute).Milliseconds() {
		t.Fatalf("unexpected panel URL parameters: %v", params)
	}
	if out.String() != "Opening "+opened+"\n" {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestQueryDashboardPanel(t *testing.T) {
	for _, tc := range []struct {
		name, result, wantError string
		args                    []string
		wantSite                string
		wantOutput              string
	}{
		{name: "saved variable", result: `{"results":{"A":{"frames":[]},"B":{"frames":[]}}}`, wantSite: "eqiad"},
		{name: "override", result: `{"results":{"A":{"frames":[]},"B":{"frames":[]}}}`, args: []string{"--var", "site=codfw"}, wantSite: "codfw"},
		{name: "backend error", result: `{"results":{"A":{"status":400,"error":"bad expression"},"B":{"frames":[]}}}`, wantError: "Grafana query A: bad expression", wantSite: "eqiad"},
		{
			name: "graph", args: []string{"--output", "graph", "--graph-width", "50"}, wantSite: "eqiad", wantOutput: "2 observations",
			result: `{"results":{"A":{"frames":[{"schema":{"fields":[{"name":"Time","type":"time"},{"name":"Value","type":"number","labels":{"site":"eqiad"}}]},"data":{"values":[[1700000000000,1700000060000,1700000120000],[4,null,12]]}}]},"B":{"frames":[]}}}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/search":
					_, _ = io.WriteString(w, `[{"uid":"example","title":"Requests Dashboard"}]`)
				case "/api/dashboards/uid/example":
					_, _ = io.WriteString(w, panelFixture)
				case "/api/datasources/uid/prom":
					_, _ = io.WriteString(w, `{"uid":"prom","type":"prometheus","jsonData":{"timeInterval":"30s"}}`)
				case "/api/ds/query":
					called = true
					if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" || r.Header.Get("User-Agent") == "" {
						t.Errorf("unexpected query request: %s %v", r.Method, r.Header)
					}
					var request panelQueryRequest
					if err := json.NewDecoder(r.Body).Decode(&request); err != nil || len(request.Queries) != 2 {
						t.Errorf("wrong query payload: %+v, %v", request, err)
						return
					}
					first, second := request.Queries[0], request.Queries[1]
					want := `rate(requests_total{site="` + tc.wantSite + `"}[120s])`
					if first.Expr != want || first.RefID != "A" || first.Datasource.UID != "prom" || !first.Range || first.Instant || first.IntervalMS != 60000 || first.MaxDataPoints != 16 {
						t.Errorf("range query = %+v", first)
					}
					if second.RefID != "B" || !second.Instant || second.Range {
						t.Errorf("instant query = %+v", second)
					}
					from, _ := strconv.ParseInt(request.From, 10, 64)
					to, _ := strconv.ParseInt(request.To, 10, 64)
					if to-from != (15 * time.Minute).Milliseconds() {
						t.Errorf("unexpected range: %s to %s", request.From, request.To)
					}
					_, _ = io.WriteString(w, tc.result)
				default:
					t.Errorf("unexpected endpoint %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			cmd := newCmd(server.URL)
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(append([]string{"dashboard", "query", "Requests Dashboard", "Requests", "--since", "15m"}, tc.args...))
			err := cmd.Execute()
			if !called || (tc.wantError == "" && err != nil) || (tc.wantError != "" && (err == nil || !strings.Contains(err.Error(), tc.wantError))) {
				t.Fatalf("query called = %v, error = %v", called, err)
			}
			if !strings.Contains(out.String(), tc.wantOutput) {
				t.Fatalf("expected %q in output %q", tc.wantOutput, out.String())
			}
		})
	}
}

func TestDashboardAndPanelTitleAmbiguity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/search" {
			t.Errorf("unexpected endpoint %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `[{"uid":"dev","title":"Request rate","folderTitle":"Development"},{"uid":"prod","title":"Request rate","folderTitle":"Production"}]`)
	}))
	defer server.Close()
	if _, err := readDashboard(t.Context(), server.URL, "Request rate"); err == nil || !strings.Contains(err.Error(), "Production / Request rate (prod)") {
		t.Fatalf("unexpected dashboard selection: %v", err)
	}
	panels := []dashboardPanel{{ID: 7, Title: "Request rate"}, {ID: 11, Title: "Request rate by site"}}
	if panel, err := selectPanel(panels, "Request rate", "Dashboard"); err != nil || panel.ID != 7 {
		t.Fatalf("exact panel title selected %+v, %v", panel, err)
	}
	if _, err := selectPanel(panels, "request", "Dashboard"); err == nil || !strings.Contains(err.Error(), "Request rate by site (11)") {
		t.Fatalf("unexpected panel selection: %v", err)
	}
}

func TestPanelQueryRequiresUnresolvedVariable(t *testing.T) {
	expr := `sum(up{site=~"$site"})`
	if _, err := expandPanelVariables(expr, nil); err == nil || !strings.Contains(err.Error(), "--var site=value") {
		t.Fatalf("unexpected error: %v", err)
	}
}
