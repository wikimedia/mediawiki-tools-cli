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
)

type pageviewsResponse struct {
	Items []pageviewItem `json:"items"`
}
type pageviewItem struct {
	Project   string `json:"project"`
	Article   string `json:"article"`
	Timestamp string `json:"timestamp"`
	Views     int64  `json:"views"`
}

func NewPageviewsCmd() *cobra.Command {
	var project, access, agent, granularity, start, end, format string
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:     "pageviews <article>",
		Short:   "Get pageview counts for a Wikimedia page",
		Example: "mw analytics pageviews 'Main Page' --project en.wikipedia.org\nmw analytics pageviews 'Wikidata' --project en.wikipedia.org --start 20261001 --end 20261017\nmw analytics pageviews 'Albert Einstein' --format csv",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "table" && format != "json" && format != "csv" {
				return fmt.Errorf("--format must be table, json, or csv")
			}
			from, to, err := pageviewRange(start, end)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()
			items, err := getPageviews(ctx, project, access, agent, args[0], granularity, from, to)
			if err != nil {
				return err
			}
			return printPageviews(cmd.OutOrStdout(), items, format)
		},
	}
	cmd.Flags().StringVar(&project, "project", "en.wikipedia.org", "Wikimedia project host, for example en.wikipedia.org or commons.wikimedia.org")
	cmd.Flags().StringVar(&access, "access", "all-access", "Access method: all-access, desktop, or mobile-app")
	cmd.Flags().StringVar(&agent, "agent", "all-agents", "Traffic agent: all-agents, user, spider, or automated")
	cmd.Flags().StringVar(&granularity, "granularity", "daily", "Data granularity: daily or monthly")
	cmd.Flags().StringVar(&start, "start", "", "Start date in YYYYMMDD format (default: 30 days ago)")
	cmd.Flags().StringVar(&end, "end", "", "End date in YYYYMMDD format (default: yesterday)")
	cmd.Flags().StringVar(&format, "format", "table", "Output format: table, json, or csv")
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
	if access != "all-access" && access != "desktop" && access != "mobile-app" {
		return nil, fmt.Errorf("invalid --access %q", access)
	}
	if agent != "all-agents" && agent != "user" && agent != "spider" && agent != "automated" {
		return nil, fmt.Errorf("invalid --agent %q", agent)
	}
	if granularity != "daily" && granularity != "monthly" {
		return nil, fmt.Errorf("invalid --granularity %q", granularity)
	}
	path := []string{"metrics", "pageviews", "per-article", project, access, agent, article, granularity, start, end}
	for i := range path {
		path[i] = url.PathEscape(path[i])
	}
	u := "https://wikimedia.org/api/rest_v1/" + strings.Join(path, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "mwcli WikimediaAnalytics/1.0 (https://www.mediawiki.org/wiki/CLI)")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		message := fmt.Sprintf("pageviews API returned %s", resp.Status)
		if retry := resp.Header.Get("Retry-After"); retry != "" {
			message += " (Retry-After: " + retry + ")"
		}
		if detail := strings.TrimSpace(string(body)); detail != "" {
			message += ": " + detail
		}
		return nil, fmt.Errorf("%s", message)
	}
	var result pageviewsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode pageviews response: %w", err)
	}
	return result.Items, nil
}

func printPageviews(out io.Writer, items []pageviewItem, format string) error {
	switch format {
	case "json":
		return json.NewEncoder(out).Encode(items)
	case "csv":
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
	default:
		if _, err := fmt.Fprintf(out, "%-12s %12s\n", "DATE", "VIEWS"); err != nil {
			return err
		}
		for _, item := range items {
			date := item.Timestamp
			if len(date) >= 8 {
				date = date[:4] + "-" + date[4:6] + "-" + date[6:8]
			}
			if _, err := fmt.Fprintf(out, "%-12s %12d\n", date, item.Views); err != nil {
				return err
			}
		}
		return nil
	}
}
