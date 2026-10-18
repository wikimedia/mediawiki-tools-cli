package analytics

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/restapi"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

type pageviewsResponse struct {
	Items []pageviewItem `json:"items"`
}
type pageviewItem struct {
	Project   string `json:"project"`
	Article   string `json:"article,omitempty"`
	Timestamp string `json:"timestamp"`
	Views     int64  `json:"views"`
}

func NewPageviewsCmd() *cobra.Command {
	var project, access, agent, granularity, start, end string
	var timeout time.Duration
	var graphWidth int
	out := output.Output{
		TableBinding: &output.TableBinding{
			Headings: []string{"Date", "Views"},
			ProcessObjects: func(objects interface{}, table *output.Table) {
				for _, item := range objects.([]pageviewItem) {
					date := item.Timestamp
					if len(date) >= 8 {
						date = date[:4] + "-" + date[4:6] + "-" + date[6:8]
					}
					table.AddRow(date, item.Views)
				}
			},
		},
	}
	cmd := &cobra.Command{
		Use:     "pageviews [article]",
		Short:   "Get pageview counts for a page or project",
		Long:    "Get pageview counts for an article, or omit the article for project totals. Dates are inclusive. All agents, including bots, are included by default; use --agent user for human traffic.",
		Example: "pageviews --project en.wikipedia.org\npageviews 'Main Page' --project en.wikipedia.org\npageviews 'Wikidata' --project en.wikipedia.org --start 20261001 --end 20261017\npageviews 'Albert Einstein' --output graph\npageviews 'Albert Einstein' --output csv\npageviews 'Albert Einstein' --output jq --format '.[].views'",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateGraphWidth(graphWidth); err != nil {
				return err
			}
			if err := validateMetricOutput(out); err != nil {
				return err
			}
			from, to, err := pageviewRange(start, end)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()
			article := ""
			if len(args) > 0 {
				article = args[0]
			}
			items, err := getPageviews(ctx, project, access, agent, article, granularity, from, to)
			if err != nil {
				return err
			}
			if items == nil {
				items = []pageviewItem{}
			}
			if out.Type == "graph" {
				points := make([]output.GraphPoint, 0, len(items))
				for _, item := range items {
					date, err := metricGraphTime(item.Timestamp)
					if err != nil {
						return err
					}
					points = append(points, output.GraphPoint{Time: date, Value: float64(item.Views)})
				}
				title := "Pageviews — " + project
				if article != "" {
					title = "Pageviews — " + article + " (" + project + ")"
				}
				return (output.Graph{Title: title, Points: points, Width: graphWidth}).Print(cmd.OutOrStdout())
			}
			if out.Type == "csv" {
				return printPageviewsCSV(cmd.OutOrStdout(), items)
			}
			out.Print(cmd, items)
			return nil
		},
	}
	cmd.Flags().StringVar(&project, "project", "en.wikipedia.org", "Wikimedia project host, for example en.wikipedia.org or commons.wikimedia.org")
	cmd.Flags().StringVar(&access, "access", "all-access", "Access method: all-access, desktop, mobile-web, or mobile-app")
	cmd.Flags().StringVar(&agent, "agent", "all-agents", "Traffic agent: all-agents, user, spider, or automated")
	cmd.Flags().StringVar(&granularity, "granularity", "daily", "Data granularity: daily or monthly")
	cmd.Flags().StringVar(&start, "start", "", "Start date in YYYYMMDD format (default: 30 days ago)")
	cmd.Flags().StringVar(&end, "end", "", "End date in YYYYMMDD format (default: yesterday)")
	out.AddFlagsWithOpts(cmd,
		output.WithDefaultTTY(output.TableType),
		output.WithDefaultPipe(output.JSONType),
		output.WithAdditionalTypes(output.Type("csv"), output.Type("graph")),
		output.WithFilterFlagDisabled(),
	)
	cmd.Flags().IntVar(&graphWidth, "graph-width", 0, "Graph width in columns (0 detects the terminal; otherwise 24–1000)")
	cmd.Flags().DurationVar(&timeout, "timeout", 20*time.Second, "HTTP request timeout")
	return cmd
}

func pageviewRange(start, end string) (string, string, error) {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	parse := func(value string) (string, error) {
		if value == "" {
			return "", nil
		}
		if len(value) != 8 {
			return "", fmt.Errorf("date %q must use YYYYMMDD", value)
		}
		if _, err := time.Parse("20060102", value); err != nil {
			return "", fmt.Errorf("invalid date %q: use YYYYMMDD", value)
		}
		return value + "00", nil
	}
	from, err := parse(start)
	if err != nil {
		return "", "", err
	}
	to, err := parse(end)
	if err != nil {
		return "", "", err
	}
	if from == "" {
		from = today.AddDate(0, 0, -30).Format("20060102") + "00"
	}
	if to == "" {
		to = today.AddDate(0, 0, -1).Format("20060102") + "00"
	}
	if from > to {
		return "", "", fmt.Errorf("--start must not be after --end")
	}
	return from, to, nil
}

func getPageviews(ctx context.Context, project, access, agent, article, granularity, start, end string) ([]pageviewItem, error) {
	if project == "" || strings.Contains(project, "/") {
		return nil, fmt.Errorf("--project must be a host such as en.wikipedia.org")
	}
	if access != "all-access" && access != "desktop" && access != "mobile-web" && access != "mobile-app" {
		return nil, fmt.Errorf("invalid --access %q", access)
	}
	if agent != "all-agents" && agent != "user" && agent != "spider" && agent != "automated" {
		return nil, fmt.Errorf("invalid --agent %q", agent)
	}
	if granularity != "daily" && granularity != "monthly" {
		return nil, fmt.Errorf("invalid --granularity %q", granularity)
	}
	path := []string{"metrics", "pageviews", "aggregate", project, access, agent, granularity, start, end}
	if article != "" {
		path = []string{"metrics", "pageviews", "per-article", project, access, agent, article, granularity, start, end}
	}
	for i := range path {
		path[i] = url.PathEscape(path[i])
	}
	resp, err := restapi.Request(ctx, http.MethodGet, restapi.BaseURL, strings.Join(path, "/"), nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := restapi.CheckStatus(resp); err != nil {
		return nil, err
	}
	var result pageviewsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode pageviews response: %w", err)
	}
	return result.Items, nil
}

func printPageviewsCSV(out io.Writer, items []pageviewItem) error {
	w := csv.NewWriter(out)
	if err := w.Write([]string{"timestamp", "project", "article", "views"}); err != nil {
		return err
	}
	for _, item := range items {
		if err := w.Write([]string{item.Timestamp, item.Project, item.Article, fmt.Sprint(item.Views)}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}
