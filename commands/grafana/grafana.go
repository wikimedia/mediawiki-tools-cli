// Package grafana reads dashboards and metrics from Wikimedia Grafana.
package grafana

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/spf13/cobra"
	apicmd "gitlab.wikimedia.org/repos/releng/cli/commands/api"
	"gitlab.wikimedia.org/repos/releng/cli/internal/restapi"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

const BaseURL = "https://grafana.wikimedia.org"

func NewCmd() *cobra.Command {
	return newCmd(BaseURL)
}

func newCmd(baseURL string) *cobra.Command {
	cmd := &cobra.Command{
		Use: "grafana", GroupID: "service",
		Short:   "Read dashboards and metrics from Wikimedia Grafana",
		Long:    "Read public Grafana dashboards, discover data sources, and run PromQL queries. The public endpoints work without credentials.",
		Example: "grafana dashboard query " + cdnPanelQueryArgs + "\ngrafana dashboard list 'MediaWiki'\ngrafana datasource list\ngrafana api /api/health",
	}

	dashboard := &cobra.Command{Use: "dashboard", Short: "Find dashboards, inspect panels, and download JSON"}
	dashboard.AddCommand(newDashboardListCmd(baseURL), newDashboardGetCmd(baseURL), newDashboardPanelsCmd(baseURL), newDashboardQueryCmd(baseURL))

	datasource := &cobra.Command{Use: "datasource", Short: "Discover Grafana data sources"}
	datasource.AddCommand(readCommand(baseURL, &cobra.Command{
		Use: "list", Short: "List public Grafana data sources", Args: cobra.NoArgs,
		Example: `list -o jq --format '.[] | select(.type == "prometheus") | {uid, name}'`,
	}, "", func([]string) (string, error) { return "/api/datasources", nil }))

	raw := apicmd.NewScopedCmd(baseURL, validateAPIRequest)
	raw.Use = "api <endpoint>"
	raw.GroupID = ""
	raw.Short = "Send a read request to the Wikimedia Grafana API"
	raw.Long = "Call a relative endpoint on Wikimedia Grafana. Use GET or HEAD for API reads, or POST /api/ds/query with --input FILE for a Grafana data source query. Use --field with --method GET for query parameters."
	raw.Example = "api /api/health\napi /api/search -X GET -f type=dash-db -f query=MediaWiki\napi /api/ds/query -X POST --input query.json"
	cmd.AddCommand(dashboard, datasource, newQueryCmd(baseURL), raw)
	return cmd
}

func validateAPIRequest(method, endpoint string) error {
	target, err := url.Parse(endpoint)
	if err != nil || target.IsAbs() || target.Host != "" || target.Fragment != "" || strings.HasPrefix(endpoint, "//") {
		return fmt.Errorf("use a relative Wikimedia Grafana API endpoint")
	}
	path := "/" + strings.TrimPrefix(target.EscapedPath(), "/")
	if !strings.HasPrefix(path, "/api/") && !strings.HasPrefix(path, "/apis/") {
		return fmt.Errorf("Grafana endpoints must start with /api/ or /apis/")
	}
	if method == http.MethodGet || method == http.MethodHead || (method == http.MethodPost && path == "/api/ds/query") {
		return nil
	}
	return fmt.Errorf("use GET or HEAD for API reads, or POST /api/ds/query for data queries")
}

func newDashboardListCmd(baseURL string) *cobra.Command {
	var limit, page int
	cmd := readCommand(baseURL, &cobra.Command{
		Use: "list [search]", Short: "Search public Grafana dashboards", Args: cobra.MaximumNArgs(1),
		Example: `list 'MediaWiki'
list --limit 20 -o jq --format '.[] | {uid, title}'`,
	}, "", func(args []string) (string, error) {
		if limit < 1 || limit > 5000 || page < 1 {
			return "", fmt.Errorf("use --limit between 1 and 5000 and --page of at least 1")
		}
		var search string
		if len(args) == 1 {
			search = args[0]
		}
		return dashboardSearchEndpoint(search, limit, page), nil
	})
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum dashboards per page (up to 5000)")
	cmd.Flags().IntVar(&page, "page", 1, "Search page number")
	return cmd
}

func newDashboardGetCmd(baseURL string) *cobra.Command {
	return readCommand(baseURL, &cobra.Command{
		Use: "get <uid>", Short: "Download a dashboard's JSON model", Args: cobra.ExactArgs(1),
		Long:    "Print the dashboard model by UID. Use dashboard list to look up UIDs, or grafana api /api/dashboards/uid/<uid> for the full response including metadata.",
		Example: "get VIfr7aank\nget VIfr7aank -o jq --format '.panels[].title'",
	}, "dashboard", dashboardEndpoint)
}

func dashboardEndpoint(args []string) (string, error) {
	if !validUID(args[0]) {
		return "", fmt.Errorf("provide a dashboard UID")
	}
	return "/api/dashboards/uid/" + url.PathEscape(args[0]), nil
}

func newQueryCmd(baseURL string) *cobra.Command {
	var datasource string
	var since, step time.Duration
	cmd := readCommand(baseURL, &cobra.Command{
		Use: "query <promql>", Short: "Run a PromQL query through a Wikimedia Grafana data source", Args: cobra.ExactArgs(1),
		Long:    "Run an instant PromQL query by default, or add --since for a time series. Use datasource list to find a Prometheus data source UID. The output is the Prometheus JSON response.",
		Example: "query 'vector(1)' --datasource 000000019\nquery 'vector(1)' --datasource 000000019 --since 15m --step 1m",
	}, "", func(args []string) (string, error) {
		if !validUID(datasource) {
			return "", fmt.Errorf("provide a Prometheus data source UID with --datasource")
		}
		if strings.TrimSpace(args[0]) == "" {
			return "", fmt.Errorf("provide a PromQL expression")
		}
		if since < 0 || step <= 0 {
			return "", fmt.Errorf("--since must be nonnegative and --step must be positive")
		}
		endpoint := "/api/datasources/proxy/uid/" + url.PathEscape(datasource) + "/api/v1/query"
		params := url.Values{"query": {args[0]}}
		if since > 0 {
			now := time.Now()
			endpoint += "_range"
			params.Set("start", now.Add(-since).UTC().Format(time.RFC3339Nano))
			params.Set("end", now.UTC().Format(time.RFC3339Nano))
			params.Set("step", strconv.FormatFloat(step.Seconds(), 'f', -1, 64))
		}
		return endpoint + "?" + params.Encode(), nil
	})
	cmd.Flags().StringVarP(&datasource, "datasource", "d", "", "Prometheus data source UID")
	cmd.Flags().DurationVar(&since, "since", 0, "Range to query, ending now (for example, 1h)")
	cmd.Flags().DurationVar(&step, "step", time.Minute, "Interval between range-query samples")
	return cmd
}

func validUID(uid string) bool {
	return uid != "" && uid != "." && uid != ".." && !strings.ContainsAny(uid, "/\\?#") &&
		!strings.ContainsFunc(uid, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) })
}

func readCommand(baseURL string, cmd *cobra.Command, field string, endpoint func([]string) (string, error)) *cobra.Command {
	var timeout time.Duration
	out := output.Output{}
	out.AddFlagsWithOpts(cmd, output.WithFilterFlagDisabled())
	cmd.Flags().DurationVar(&timeout, "timeout", 20*time.Second, "HTTP request timeout")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := validateJSONOutput(out); err != nil {
			return err
		}
		if timeout <= 0 {
			return fmt.Errorf("--timeout must be positive")
		}
		path, err := endpoint(args)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()
		data, err := readGrafanaJSON(ctx, baseURL, path, field)
		if err != nil {
			return err
		}
		return output.PrintJSONChecked(data, out.Format, out.Type == string(output.JQType), cmd.OutOrStdout())
	}
	return cmd
}

func readGrafanaJSON(ctx context.Context, baseURL, path, field string) (json.RawMessage, error) {
	response, err := restapi.Request(ctx, http.MethodGet, baseURL, path, nil, nil)
	if err != nil {
		return nil, err
	}
	return decodeGrafanaJSON(response, field)
}

func decodeGrafanaJSON(response *http.Response, field string) (json.RawMessage, error) {
	defer response.Body.Close()
	if err := restapi.CheckStatus(response); err != nil {
		return nil, err
	}
	var data json.RawMessage
	if err := json.NewDecoder(response.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode Grafana JSON: %w", err)
	}
	if field != "" {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(data, &fields); err != nil {
			return nil, fmt.Errorf("decode Grafana object: %w", err)
		}
		data = fields[field]
		if len(data) == 0 || string(data) == "null" {
			return nil, fmt.Errorf("Grafana response is missing %q", field)
		}
	}
	return data, nil
}

func validateJSONOutput(out output.Output) error {
	if out.Type != string(output.JSONType) && out.Type != string(output.JQType) {
		return fmt.Errorf("--output must be json or jq")
	}
	if out.Type == string(output.JQType) && out.Format == "" {
		return fmt.Errorf("--output jq requires a --format expression")
	}
	return output.ValidateJSONFormat(out.Format)
}
