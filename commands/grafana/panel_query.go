package grafana

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/restapi"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

type panelQueryRequest struct {
	From    string            `json:"from"`
	To      string            `json:"to"`
	Queries []prometheusQuery `json:"queries"`
}

type prometheusQuery struct {
	RefID         string      `json:"refId"`
	Datasource    panelSource `json:"datasource"`
	Expr          string      `json:"expr"`
	Instant       bool        `json:"instant"`
	Range         bool        `json:"range"`
	Format        string      `json:"format,omitempty"`
	Interval      string      `json:"interval,omitempty"`
	IntervalMS    int64       `json:"intervalMs"`
	MaxDataPoints int64       `json:"maxDataPoints"`
}

type prometheusSource struct {
	UID      string `json:"uid"`
	Type     string `json:"type"`
	JSONData struct {
		TimeInterval string `json:"timeInterval"`
	} `json:"jsonData"`
}

const cdnPanelQueryArgs = "'CDN Frontend Traffic' 'Varnish status' --since 15m --var site=eqiad --var cache_type=text --var status_type=2"

var panelVariable = regexp.MustCompile(`\$([a-zA-Z_][a-zA-Z_0-9]*)|\$\{([a-zA-Z_][a-zA-Z_0-9]*)(?::([^}]+))?\}|\[\[([a-zA-Z_][a-zA-Z_0-9]*)(?::([^\]]+))?\]\]`)

func newDashboardQueryCmd(baseURL string, openURL func(string) error) *cobra.Command {
	var datasource string
	var overrides []string
	var since, step, timeout time.Duration
	var graphWidth int
	const graphOutput = output.Type("graph")
	out := output.Output{}
	cmd := &cobra.Command{
		Use: "query <dashboard> <panel>", Short: "Retrieve data from a dashboard's Prometheus panel", Args: cobra.ExactArgs(2),
		Long:    "Run the visible Prometheus queries saved on a panel and return Grafana's data frames. Use --output graph to plot numeric time series or --output web to open the panel in Grafana with the requested time range and variables. Use titles or IDs for the dashboard and panel.",
		Example: "query " + cdnPanelQueryArgs + " --output graph\nquery " + cdnPanelQueryArgs + " --output web\nquery 'MediaWiki LinksUpdate' 'RefreshLinksJob good outcome' --since 15m",
		RunE: func(cmd *cobra.Command, args []string) error {
			graph := out.Type == string(graphOutput)
			web := out.Type == string(output.WebType)
			if graph || web {
				if out.Format != "" {
					return fmt.Errorf("--format requires JSON or jq output")
				}
			} else if err := validateJSONOutput(out); err != nil {
				return err
			}
			if err := output.ValidateGraphWidth(graphWidth); err != nil {
				return err
			}
			if since < time.Millisecond || step < time.Millisecond || timeout <= 0 {
				return fmt.Errorf("--since and --step must be at least 1ms and --timeout must be positive")
			}
			if datasource != "" && !validUID(datasource) {
				return fmt.Errorf("--datasource must be a data source UID")
			}
			if web && datasource != "" {
				return fmt.Errorf("--output web uses the dashboard's data source; use --var to set a dashboard data source variable")
			}
			selectedVars, err := parseVariableOverrides(overrides)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()
			dashboard, err := loadDashboard(ctx, baseURL, args[0])
			if err != nil {
				return err
			}
			panel, err := selectPanel(flattenPanels(dashboard.Panels), args[1], args[0])
			if err != nil {
				return err
			}
			if web {
				link, err := panelWebURL(baseURL, dashboard.UID, panel.ID, since, selectedVars)
				if err != nil {
					return err
				}
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), "Opening", link); err != nil {
					return err
				}
				return openURL(link)
			}
			vars := panelVariables(dashboard, selectedVars)
			result, err := queryPanel(ctx, baseURL, panel, datasource, vars, since, step)
			if err != nil {
				return err
			}
			if graph {
				title := panel.Title
				if title == "" {
					title = fmt.Sprintf("Panel %d", panel.ID)
				} else if expanded, err := expandPanelVariables(title, vars); err == nil {
					title = expanded
				}
				return printPanelGraphs(cmd.OutOrStdout(), title, result, graphWidth)
			}
			return output.PrintJSONChecked(result, out.Format, out.Type == string(output.JQType), cmd.OutOrStdout())
		},
	}
	cmd.Flags().StringVarP(&datasource, "datasource", "d", "", "Override the Prometheus data source UID")
	cmd.Flags().StringArrayVar(&overrides, "var", nil, "Dashboard variable as name=value (repeatable)")
	cmd.Flags().DurationVar(&since, "since", time.Hour, "Time range ending now")
	cmd.Flags().DurationVar(&step, "step", time.Minute, "Interval between range-query samples")
	cmd.Flags().DurationVar(&timeout, "timeout", 20*time.Second, "Timeout for fetching and querying the panel")
	cmd.Flags().IntVar(&graphWidth, "graph-width", 0, "Graph width in columns (0 detects the terminal; otherwise 24–1000)")
	out.AddFlagsWithOpts(cmd, output.WithAdditionalTypes(graphOutput, output.WebType), output.WithFilterFlagDisabled())
	return cmd
}

func panelWebURL(baseURL, uid string, panelID int, since time.Duration, vars map[string]string) (string, error) {
	if !validUID(uid) || panelID < 1 {
		return "", fmt.Errorf("Grafana did not return a valid dashboard UID and panel ID")
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	link := base.JoinPath("d", url.PathEscape(uid))
	now := time.Now()
	params := link.Query()
	params.Set("viewPanel", strconv.Itoa(panelID))
	params.Set("from", strconv.FormatInt(now.Add(-since).UnixMilli(), 10))
	params.Set("to", strconv.FormatInt(now.UnixMilli(), 10))
	for name, value := range vars {
		params.Set("var-"+name, value)
	}
	link.RawQuery = params.Encode()
	return link.String(), nil
}

func panelVariables(dashboard dashboardModel, overrides map[string]string) map[string]string {
	values := map[string]string{}
	for _, variable := range dashboard.Templating.List {
		var value string
		if json.Unmarshal(variable.Current.Value, &value) != nil {
			var selected []string
			if json.Unmarshal(variable.Current.Value, &selected) == nil && len(selected) == 1 {
				value = selected[0]
			}
		}
		if value != "" && value != "$__all" {
			values[variable.Name] = value
		}
	}
	for name, value := range overrides {
		values[name] = value
	}
	return values
}

func parseVariableOverrides(overrides []string) (map[string]string, error) {
	values := map[string]string{}
	for _, override := range overrides {
		name, value, ok := strings.Cut(override, "=")
		if !ok || !validVariableName.MatchString(name) || strings.HasPrefix(name, "__") {
			return nil, fmt.Errorf("--var must supply a dashboard variable as name=value")
		}
		values[name] = value
	}
	return values, nil
}

var validVariableName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z_0-9]*$`)

func expandPanelVariables(template string, vars map[string]string) (string, error) {
	var err error
	result := panelVariable.ReplaceAllStringFunc(template, func(token string) string {
		part := panelVariable.FindStringSubmatch(token)
		name, format := part[1], ""
		if part[2] != "" {
			name, format = part[2], part[3]
		} else if part[4] != "" {
			name, format = part[4], part[5]
		}
		if format != "" && format != "raw" {
			if err == nil {
				err = fmt.Errorf("variable %s uses Grafana's %q format, which requires the panel's Query inspector", name, format)
			}
			return token
		}
		if value, ok := vars[name]; ok {
			return value
		}
		if err == nil {
			if strings.HasPrefix(name, "__") {
				err = fmt.Errorf("Grafana variable %s requires the panel's Query inspector", token)
			} else {
				err = fmt.Errorf("set dashboard variable %s with --var %s=value", token, name)
			}
		}
		return token
	})
	return result, err
}

func queryPanel(ctx context.Context, baseURL string, panel dashboardPanel, datasource string, vars map[string]string, since, step time.Duration) (json.RawMessage, error) {
	now := time.Now()
	request := panelQueryRequest{
		From: strconv.FormatInt(now.Add(-since).UnixMilli(), 10),
		To:   strconv.FormatInt(now.UnixMilli(), 10),
	}
	vars["__from"], vars["__to"] = request.From, request.To
	vars["__range"] = prometheusDuration(since)
	vars["__range_s"] = strconv.FormatFloat(since.Seconds(), 'f', -1, 64)
	vars["__range_ms"] = strconv.FormatInt(since.Milliseconds(), 10)
	vars["__interval"] = prometheusDuration(step)
	vars["__interval_ms"] = strconv.FormatInt(step.Milliseconds(), 10)
	sources := map[panelSource]prometheusSource{}
	for _, target := range panel.Targets {
		if target.Hidden {
			continue
		}
		expr := target.expression()
		source := target.source(panel)
		if expr == "" || (source.Type != "" && source.Type != "prometheus") {
			return nil, fmt.Errorf("panel %d query %s requires a Prometheus expression; use the panel's Query inspector for other data sources", panel.ID, target.RefID)
		}
		if datasource != "" {
			source = panelSource{UID: datasource}
		}
		info, ok := sources[source]
		if !ok {
			var err error
			info, err = loadPrometheusSource(ctx, baseURL, source, vars)
			if err != nil {
				return nil, fmt.Errorf("query %s: %w", target.RefID, err)
			}
			sources[source] = info
		}
		if strings.Contains(expr, "__rate_interval") {
			scrape := target.MinStep
			if scrape == "" {
				scrape = info.JSONData.TimeInterval
			}
			scrapeInterval := 15 * time.Second
			if scrape != "" {
				var err error
				scrape, err = expandPanelVariables(scrape, vars)
				if err != nil {
					return nil, fmt.Errorf("query %s: %w", target.RefID, err)
				}
				scrapeInterval, err = time.ParseDuration(scrape)
				if err != nil || scrapeInterval < time.Millisecond {
					return nil, fmt.Errorf("query %s has an invalid Prometheus scrape interval %q", target.RefID, scrape)
				}
			}
			rateInterval := max(step+scrapeInterval, 4*scrapeInterval)
			vars["__rate_interval"] = prometheusDuration(rateInterval)
			vars["__rate_interval_ms"] = strconv.FormatInt(rateInterval.Milliseconds(), 10)
		}
		expr, err := expandPanelVariables(expr, vars)
		if err != nil {
			return nil, fmt.Errorf("query %s: %w", target.RefID, err)
		}
		query := prometheusQuery{
			RefID: target.RefID, Datasource: panelSource{UID: info.UID, Type: info.Type}, Expr: expr,
			Instant: target.Instant, Range: !target.Instant, Format: target.Format,
			Interval: prometheusDuration(step), IntervalMS: step.Milliseconds(), MaxDataPoints: int64(since/step) + 1,
		}
		if target.Range != nil {
			query.Range = *target.Range
		}
		request.Queries = append(request.Queries, query)
	}
	if len(request.Queries) == 0 {
		return nil, fmt.Errorf("panel %d has no visible queries", panel.ID)
	}
	body, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	resp, err := restapi.Request(ctx, http.MethodPost, baseURL, "/api/ds/query", http.Header{"Content-Type": {"application/json"}}, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	data, err := decodeGrafanaJSON(resp, "")
	if err != nil {
		return nil, err
	}
	var result struct {
		Results map[string]struct {
			Status int    `json:"status"`
			Error  string `json:"error"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode Grafana query results: %w", err)
	}
	for _, query := range request.Queries {
		entry, ok := result.Results[query.RefID]
		if !ok {
			return nil, fmt.Errorf("Grafana did not return a result for query %s", query.RefID)
		}
		if entry.Error != "" {
			return nil, fmt.Errorf("Grafana query %s: %s", query.RefID, entry.Error)
		}
		if entry.Status >= 400 {
			return nil, fmt.Errorf("Grafana query %s returned status %d", query.RefID, entry.Status)
		}
	}
	return data, nil
}

func loadPrometheusSource(ctx context.Context, baseURL string, source panelSource, vars map[string]string) (prometheusSource, error) {
	var info prometheusSource
	var endpoint string
	switch {
	case source.UID != "":
		uid, err := expandPanelVariables(source.UID, vars)
		if err != nil || !validUID(uid) {
			return info, fmt.Errorf("select a Prometheus data source with --datasource")
		}
		endpoint = "/api/datasources/uid/" + url.PathEscape(uid)
	case source.Name != "":
		name, err := expandPanelVariables(source.Name, vars)
		if err != nil {
			return info, err
		}
		endpoint = "/api/datasources/name/" + url.PathEscape(name)
	default:
		return info, fmt.Errorf("select a Prometheus data source with --datasource")
	}
	data, err := readGrafanaJSON(ctx, baseURL, endpoint, "")
	if err != nil {
		return info, err
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return info, fmt.Errorf("decode Grafana data source: %w", err)
	}
	if !validUID(info.UID) || info.Type != "prometheus" {
		return info, fmt.Errorf("select a Prometheus data source with --datasource")
	}
	return info, nil
}

func prometheusDuration(duration time.Duration) string {
	if duration%time.Second == 0 {
		return strconv.FormatInt(int64(duration/time.Second), 10) + "s"
	}
	return strconv.FormatInt(duration.Milliseconds(), 10) + "ms"
}
