package analytics

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/restapi"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

type metricOptions struct {
	graphWidth                                                                 int
	project, editorType, pageType, activity, access, referer, mediaType, agent string
	granularity, start, end, date, month                                       string
	limit                                                                      int
	timeout                                                                    time.Duration
}

type metricColumn struct{ heading, key string }

type metricDefinition struct {
	name, short, example string
	columns              []metricColumn
	ranking              bool
}

func newMetricCmd(def metricDefinition) *cobra.Command {
	opts := metricOptions{}
	out := output.Output{TableBinding: &output.TableBinding{}}
	for _, column := range def.columns {
		out.TableBinding.Headings = append(out.TableBinding.Headings, column.heading)
	}
	out.TableBinding.ProcessObjects = func(objects interface{}, table *output.Table) {
		for _, row := range objects.([]map[string]interface{}) {
			cells := make([]string, len(def.columns))
			for i, col := range def.columns {
				cells[i] = metricCell(row, col.key, true)
			}
			table.AddRowS(cells...)
		}
	}
	cmd := &cobra.Command{Use: def.name, Short: def.short, Example: def.example, Args: cobra.NoArgs}
	if !def.ranking {
		cmd.Long = def.short + ". Start and end dates are inclusive and use UTC; periods without activity may be omitted by the API."
	}
	if def.name == "edits" {
		cmd.Use = "edits [article]"
		cmd.Args = cobra.MaximumNArgs(1)
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := validateGraphWidth(opts.graphWidth); err != nil {
			return err
		}
		if err := validateMetricOutput(out); err != nil {
			return err
		}
		if opts.limit < 0 {
			return fmt.Errorf("--limit must be zero or greater")
		}
		article := ""
		if len(args) > 0 {
			article = strings.ReplaceAll(args[0], " ", "_")
		}
		if article != "" && cmd.Flags().Changed("page-type") {
			return fmt.Errorf("--page-type only applies to project totals")
		}
		path, err := opts.path(def.name, article, def.ranking)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), opts.timeout)
		defer cancel()
		rows, err := fetchMetrics(ctx, path)
		// Daily rankings can lag behind the calendar. Only an omitted date
		// permits looking back; explicit dates and other failures stay exact.
		if def.ranking && !cmd.Flags().Changed("date") && !cmd.Flags().Changed("month") {
			date, _ := time.Parse("2006/01/02", strings.Join(path[len(path)-3:], "/"))
			for daysBack := 1; daysBack < 7 && err != nil; daysBack++ {
				var apiErr *restapi.HTTPError
				if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
					break
				}
				date = date.AddDate(0, 0, -1)
				path = append(path[:len(path)-3], date.Format("2006"), date.Format("01"), date.Format("02"))
				rows, err = fetchMetrics(ctx, path)
				if err == nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Using rankings for %s (newer data is unavailable).\n", date.Format("2006-01-02"))
				}
			}
		}
		if err != nil {
			return err
		}
		if def.ranking && opts.limit > 0 && len(rows) > opts.limit {
			rows = rows[:opts.limit]
		}
		if out.Type == "graph" {
			return printMetricGraph(cmd, rows, def, opts)
		}
		if out.Type == "csv" {
			writer := csv.NewWriter(cmd.OutOrStdout())
			headings := make([]string, len(def.columns))
			for i, col := range def.columns {
				headings[i] = col.key
			}
			if err := writer.Write(headings); err != nil {
				return err
			}
			for _, row := range rows {
				cells := make([]string, len(def.columns))
				for i, col := range def.columns {
					cells[i] = metricCell(row, col.key, false)
				}
				if err := writer.Write(cells); err != nil {
					return err
				}
			}
			writer.Flush()
			return writer.Error()
		}
		out.Print(cmd, rows)
		return nil
	}
	flags := cmd.Flags()
	if def.name != "mediarequests" && def.name != "top" {
		flags.StringVar(&opts.project, "project", "en.wikipedia.org", "Wikimedia project host, for example en.wikipedia.org")
	}
	switch def.name {
	case "edits", "editors", "new-pages", "top-edited":
		flags.StringVar(&opts.editorType, "editor-type", "all-editor-types", "Editor type: all-editor-types, anonymous, group-bot, name-bot, or user")
		flags.StringVar(&opts.pageType, "page-type", "all-page-types", "Page type: all-page-types, content, or non-content")
	}
	if def.name == "editors" {
		flags.StringVar(&opts.activity, "activity-level", "all-activity-levels", "Activity: all-activity-levels, 1..4-edits, 5..24-edits, 25..99-edits, or 100..-edits")
	}
	if def.name == "top-pages" {
		flags.StringVar(&opts.access, "access", "all-access", "Access: all-access, desktop, mobile-web, or mobile-app")
	}
	if def.name == "devices" {
		flags.StringVar(&opts.access, "access-site", "all-sites", "Access: all-sites, desktop-site, or mobile-site")
	}
	if def.name == "mediarequests" || def.name == "top" {
		flags.StringVar(&opts.referer, "referer", "all-referers", "Request origin: Wikimedia project host, all-referers, internal, external, search-engine, unknown, or none")
		flags.StringVar(&opts.mediaType, "media-type", "all-media-types", "Media type: all-media-types, image, video, audio, document, or other")
		if !def.ranking {
			flags.StringVar(&opts.agent, "agent", "all-agents", "Agent: all-agents, user, or spider")
		}
	}
	if def.ranking {
		flags.StringVar(&opts.date, "date", "", "Day in YYYYMMDD format (default: latest available in the past 7 days)")
		flags.StringVar(&opts.month, "month", "", "Whole month in YYYYMM format, instead of --date")
		cmd.MarkFlagsMutuallyExclusive("date", "month")
		flags.IntVar(&opts.limit, "limit", 20, "Maximum results (0 means all available results)")
	} else {
		flags.StringVar(&opts.granularity, "granularity", "daily", "Data granularity: daily or monthly")
		flags.StringVar(&opts.start, "start", "", "Start date in YYYYMMDD format (default: 30 days ago)")
		flags.StringVar(&opts.end, "end", "", "End date in YYYYMMDD format (default: yesterday)")
	}
	flags.IntVar(&opts.graphWidth, "graph-width", 0, "Graph width in columns (0 detects the terminal; otherwise 24–1000)")
	flags.DurationVar(&opts.timeout, "timeout", 20*time.Second, "HTTP request timeout")
	out.AddFlagsWithOpts(cmd, output.WithDefaultTTY(output.TableType), output.WithDefaultPipe(output.JSONType), output.WithAdditionalTypes(output.Type("csv"), output.Type("graph")), output.WithFilterFlagDisabled())
	return cmd
}

func validateMetricOutput(out output.Output) error {
	switch out.Type {
	case string(output.AutoType), string(output.TableType), string(output.JSONType), string(output.JQType), "csv", "graph":
	default:
		return fmt.Errorf("--output must be table, json, jq, csv, or graph")
	}
	if out.Type == string(output.JQType) && out.Format == "" {
		return fmt.Errorf("--output jq requires a --format expression")
	}
	return nil
}

func checkChoice(flag, value string, allowed ...string) error {
	for _, v := range allowed {
		if v == value {
			return nil
		}
	}
	return fmt.Errorf("--%s must be one of: %s", flag, strings.Join(allowed, ", "))
}

func (o metricOptions) path(name, article string, ranking bool) ([]string, error) {
	if name != "mediarequests" && name != "top" && (o.project == "" || strings.ContainsAny(o.project, "/\\?# \t\r\n")) {
		return nil, fmt.Errorf("--project must be a Wikimedia project host or project family")
	}
	if o.editorType != "" {
		if err := checkChoice("editor-type", o.editorType, "all-editor-types", "anonymous", "group-bot", "name-bot", "user"); err != nil {
			return nil, err
		}
		if err := checkChoice("page-type", o.pageType, "all-page-types", "content", "non-content"); err != nil {
			return nil, err
		}
	}
	if name == "editors" {
		if err := checkChoice("activity-level", o.activity, "all-activity-levels", "1..4-edits", "5..24-edits", "25..99-edits", "100..-edits"); err != nil {
			return nil, err
		}
	}
	if name == "devices" {
		if err := checkChoice("access-site", o.access, "all-sites", "desktop-site", "mobile-site"); err != nil {
			return nil, err
		}
	}
	if name == "top-pages" {
		if err := checkChoice("access", o.access, "all-access", "desktop", "mobile-web", "mobile-app"); err != nil {
			return nil, err
		}
	}
	if name == "mediarequests" || name == "top" {
		if o.referer == "" || strings.ContainsAny(o.referer, "/\\?# \t\r\n") {
			return nil, fmt.Errorf("invalid --referer")
		}
		if err := checkChoice("media-type", o.mediaType, "all-media-types", "image", "video", "audio", "document", "other"); err != nil {
			return nil, err
		}
		if !ranking {
			if err := checkChoice("agent", o.agent, "all-agents", "user", "spider"); err != nil {
				return nil, err
			}
		}
	}
	var path []string
	switch name {
	case "edits":
		if article == "" {
			path = []string{"edits", "aggregate", o.project, o.editorType, o.pageType}
		} else {
			path = []string{"edits", "per-page", o.project, article, o.editorType}
		}
	case "editors":
		path = []string{"editors", "aggregate", o.project, o.editorType, o.pageType, o.activity}
	case "new-pages":
		path = []string{"edited-pages", "new", o.project, o.editorType, o.pageType}
	case "devices":
		path = []string{"unique-devices", o.project, o.access}
	case "mediarequests":
		path = []string{"mediarequests", "aggregate", o.referer, o.mediaType, o.agent}
	case "top-pages":
		path = []string{"pageviews", "top", o.project, o.access}
	case "top-edited":
		path = []string{"edited-pages", "top-by-edits", o.project, o.editorType, o.pageType}
	case "top":
		path = []string{"mediarequests", "top", o.referer, o.mediaType}
	default:
		return nil, fmt.Errorf("unknown analytics metric %q", name)
	}
	if ranking {
		date := o.date
		if o.month != "" {
			parsed, err := time.Parse("200601", o.month)
			if err != nil {
				return nil, fmt.Errorf("--month must use YYYYMM")
			}
			return append(path, parsed.Format("2006"), parsed.Format("01"), "all-days"), nil
		}
		if date == "" {
			date = time.Now().UTC().AddDate(0, 0, -1).Format("20060102")
		}
		parsed, err := time.Parse("20060102", date)
		if err != nil {
			return nil, fmt.Errorf("--date must use YYYYMMDD")
		}
		return append(path, parsed.Format("2006"), parsed.Format("01"), parsed.Format("02")), nil
	}
	if err := checkChoice("granularity", o.granularity, "daily", "monthly"); err != nil {
		return nil, err
	}
	start, end, err := pageviewRange(o.start, o.end)
	if err != nil {
		return nil, err
	}
	endDate := end[:8]
	// Editing services use an exclusive upper bound; the CLI consistently
	// accepts an inclusive --end, as pageviews, devices and media requests do.
	if name == "edits" || name == "editors" || name == "new-pages" {
		parsed, _ := time.Parse("20060102", endDate)
		endDate = parsed.AddDate(0, 0, 1).Format("20060102")
	}
	return append(path, o.granularity, start[:8], endDate), nil
}

// Expand time-series and ranking arrays into rows, retaining their context.
func metricRows(items []map[string]interface{}) ([]map[string]interface{}, error) {
	rows := make([]map[string]interface{}, 0)
	for _, item := range items {
		nested := ""
		for _, key := range []string{"results", "articles", "top", "files"} {
			if _, ok := item[key]; ok {
				nested = key
				break
			}
		}
		if nested == "" {
			rows = append(rows, item)
			continue
		}
		children, ok := item[nested].([]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid analytics %s array", nested)
		}
		merged := make([]map[string]interface{}, 0, len(children))
		for _, child := range children {
			fields, ok := child.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid analytics result row")
			}
			row := make(map[string]interface{}, len(item)+len(fields))
			for k, v := range item {
				if k != nested {
					row[k] = v
				}
			}
			for k, v := range fields {
				row[k] = v
			}
			merged = append(merged, row)
		}
		expanded, err := metricRows(merged)
		if err != nil {
			return nil, err
		}
		rows = append(rows, expanded...)
	}
	return rows, nil
}

func fetchMetrics(ctx context.Context, path []string) ([]map[string]interface{}, error) {
	segments := append([]string{"metrics"}, path...)
	for i := range segments {
		segments[i] = url.PathEscape(segments[i])
	}
	resp, err := restapi.Request(ctx, http.MethodGet, restapi.BaseURL, strings.Join(segments, "/"), nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := restapi.CheckStatus(resp); err != nil {
		return nil, err
	}
	var data struct {
		Items []map[string]interface{} `json:"items"`
	}
	decoder := json.NewDecoder(resp.Body)
	decoder.UseNumber()
	if err := decoder.Decode(&data); err != nil {
		return nil, fmt.Errorf("decode analytics response: %w", err)
	}
	return metricRows(data.Items)
}

func metricCell(row map[string]interface{}, key string, display bool) string {
	value, ok := row[key]
	if !ok || value == nil {
		return ""
	}
	text := fmt.Sprint(value)
	if display && key == "timestamp" && len(text) >= 8 && !strings.Contains(text, "-") {
		return text[:4] + "-" + text[4:6] + "-" + text[6:8]
	}
	if display && key == "timestamp" && len(text) >= 10 {
		return text[:10]
	}
	return text
}
