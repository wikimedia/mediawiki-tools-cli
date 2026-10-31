package tools

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/restapi"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

const toolviewsURL = "https://toolviews.toolforge.org/api/v1/"

type toolviewRow struct {
	Date string `json:"date"`
	Tool string `json:"tool"`
	Hits int64  `json:"hits"`
}

func toolviewsRequest(ctx context.Context, endpoint string, result any) error {
	resp, err := restapi.Request(ctx, http.MethodGet, toolviewsURL, endpoint, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := restapi.CheckStatus(resp); err != nil {
		return err
	}
	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("decode Toolviews response: %w", err)
	}
	return nil
}

// fetchToolviewsTools is shared by list output and live shell completion.
func fetchToolviewsTools(ctx context.Context) ([]string, error) {
	var response struct {
		Results []string `json:"results"`
	}
	if err := toolviewsRequest(ctx, "tools", &response); err != nil {
		return nil, err
	}
	if response.Results == nil {
		response.Results = []string{}
	}
	sort.Strings(response.Results)
	return response.Results, nil
}

func completeToolviewsTools(cmd *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Second)
	defer cancel()
	tools, err := cachedToolviewsNames(ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	matches := []string{}
	for _, tool := range tools {
		if strings.HasPrefix(tool, prefix) {
			matches = append(matches, tool)
		}
	}
	return matches, cobra.ShellCompDirectiveNoFileComp
}

func toolviewsRange(date, start, end string, now time.Time) (string, string, error) {
	if date != "" {
		if start != "" || end != "" {
			return "", "", fmt.Errorf("--date cannot be combined with --start or --end")
		}
		start, end = date, date
	}
	today := now.UTC().Truncate(24 * time.Hour)
	if start == "" {
		start = today.AddDate(0, 0, -30).Format(time.DateOnly)
	}
	if end == "" {
		end = today.AddDate(0, 0, -1).Format(time.DateOnly)
	}
	for _, value := range []string{start, end} {
		if _, err := time.Parse(time.DateOnly, value); err != nil {
			return "", "", fmt.Errorf("invalid date %q: use YYYY-MM-DD", value)
		}
	}
	if start > end {
		return "", "", fmt.Errorf("--start must not be after --end")
	}
	return start, end, nil
}

func fetchToolviews(ctx context.Context, tool, start, end string) ([]toolviewRow, error) {
	path := ""
	if tool != "" {
		path += "tool/" + url.PathEscape(tool) + "/"
	}
	rows := []toolviewRow{}
	if start == end {
		var response struct {
			Results map[string]int64 `json:"results"`
		}
		if err := toolviewsRequest(ctx, path+"day/"+start, &response); err != nil {
			return nil, err
		}
		for name, hits := range response.Results {
			rows = append(rows, toolviewRow{start, name, hits})
		}
	} else {
		var response struct {
			Results map[string]map[string]int64 `json:"results"`
		}
		if err := toolviewsRequest(ctx, path+"daily/"+start+"/"+end, &response); err != nil {
			return nil, err
		}
		for date, tools := range response.Results {
			for name, hits := range tools {
				rows = append(rows, toolviewRow{date, name, hits})
			}
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Date != rows[j].Date {
			return rows[i].Date < rows[j].Date
		}
		if start == end && rows[i].Hits != rows[j].Hits {
			return rows[i].Hits > rows[j].Hits
		}
		return rows[i].Tool < rows[j].Tool
	})
	return rows, nil
}

func newToolviewsCmd() *cobra.Command {
	var date, start, end string
	var width int
	var timeout time.Duration
	out := output.Output{TableBinding: &output.TableBinding{
		Headings: []string{"Date", "Tool", "Hits"},
		ProcessObjects: func(objects interface{}, table *output.Table) {
			for _, row := range objects.([]toolviewRow) {
				table.AddRow(row.Date, row.Tool, row.Hits)
			}
		},
	}}
	command := &cobra.Command{
		Use: "toolviews [tool]", Aliases: []string{"views"},
		Short:             "Get Toolforge web traffic statistics from Toolviews",
		Long:              "Query public Toolviews webserver statistics without authentication. Omit the tool to query all tools. Dates are inclusive and default to the last 30 completed UTC days. Hits include automated traffic and are not counts of people. Missing observations are omitted, not filled with zeroes. JSON and CSV report counts in the hits field.",
		Example:           "toolviews toolviews --output graph\ntoolviews --date 2026-10-27 --output graph\ntoolviews toolviews --start 2026-10-01 --end 2026-10-07\ntoolviews list\ntoolviews --date \"$(date -u -d yesterday +%F)\" --output table | head -n 11",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeToolviewsTools,
		RunE: func(cmd *cobra.Command, args []string) error {
			if width != 0 && (width < 24 || width > 1000) {
				return fmt.Errorf("--graph-width must be 0 (automatic), or between 24 and 1000")
			}
			if timeout <= 0 {
				return fmt.Errorf("--timeout must be positive")
			}
			if out.Type == "jq" && out.Format == "" {
				return fmt.Errorf("--output jq requires a --format expression")
			}
			switch out.Type {
			case "auto", "json", "jq", "table", "csv", "graph":
			default:
				return fmt.Errorf("unsupported output format %q", out.Type)
			}
			from, to, err := toolviewsRange(date, start, end, time.Now())
			if err != nil {
				return err
			}
			tool := ""
			if len(args) > 0 {
				tool = args[0]
				if strings.TrimSpace(tool) == "" || strings.ContainsAny(tool, "/\\?# \t\r\n") || tool == "." || tool == ".." {
					return fmt.Errorf("tool must be a Toolforge tool name")
				}
			}
			if out.Type == "graph" && tool == "" && from != to {
				return fmt.Errorf("a time-series graph requires a tool; use --date for an all-tools bar chart")
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()
			rows, err := fetchToolviews(ctx, tool, from, to)
			if err != nil {
				return err
			}
			heading := "Hits"
			out.TableBinding.Headings[2] = heading
			if out.Type == "graph" {
				subject := tool
				if subject == "" {
					subject = from
				}
				graph := output.Graph{Title: heading + " — " + subject, Width: width, Bars: from == to}
				for _, row := range rows {
					day, err := time.Parse(time.DateOnly, row.Date)
					if err != nil {
						return fmt.Errorf("invalid Toolviews observation date %q", row.Date)
					}
					graph.Points = append(graph.Points, output.GraphPoint{Time: day, Label: row.Tool, Value: float64(row.Hits)})
				}
				return graph.Print(cmd.OutOrStdout())
			}
			if out.Type == "csv" {
				writer := csv.NewWriter(cmd.OutOrStdout())
				if err := writer.Write([]string{"date", "tool", "hits"}); err != nil {
					return err
				}
				for _, row := range rows {
					if err := writer.Write([]string{row.Date, row.Tool, strconv.FormatInt(row.Hits, 10)}); err != nil {
						return err
					}
				}
				writer.Flush()
				return writer.Error()
			}
			out.Print(cmd, rows)
			return nil
		},
	}
	command.Flags().StringVar(&date, "date", "", "Single UTC date (YYYY-MM-DD)")
	command.Flags().StringVar(&start, "start", "", "First UTC date (YYYY-MM-DD; default: 30 days ago)")
	command.Flags().StringVar(&end, "end", "", "Last UTC date (YYYY-MM-DD; default: yesterday)")
	command.Flags().IntVar(&width, "graph-width", 0, "Graph width in columns (0 detects the terminal; otherwise 24–1000)")
	command.Flags().DurationVar(&timeout, "timeout", 20*time.Second, "HTTP request timeout")
	out.AddFlagsWithOpts(command, output.WithDefaultTTY(output.TableType), output.WithDefaultPipe(output.JSONType), output.WithAdditionalTypes(output.Type("csv"), output.Type("graph")), output.WithFilterFlagDisabled())
	command.AddCommand(newToolviewsListCmd(), newToolviewsRefreshCmd())
	return command
}

func newToolviewsListCmd() *cobra.Command {
	var timeout time.Duration
	var refresh bool
	out := output.Output{TableBinding: &output.TableBinding{Headings: []string{"Tool"}, ProcessObjects: func(objects interface{}, table *output.Table) {
		for _, tool := range objects.([]string) {
			table.AddRow(tool)
		}
	}}}
	command := &cobra.Command{Use: "list", Short: "List tool names known to Toolviews", Long: "List tool names present in the public Toolviews dataset, sorted alphabetically. Names are cached on disk and shared with shell completion. After 24 hours, cached names are returned immediately while a background request refreshes them. Use --refresh to fetch a fresh list synchronously. No Toolforge login is required.", Example: "list\nlist --output json", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if timeout <= 0 {
			return fmt.Errorf("--timeout must be positive")
		}
		if out.Type == "jq" && out.Format == "" {
			return fmt.Errorf("--output jq requires a --format expression")
		}
		switch out.Type {
		case "auto", "json", "jq", "table":
		default:
			return fmt.Errorf("unsupported output format %q", out.Type)
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()
		var names []string
		var err error
		if refresh {
			names, err = toolviewsNamesCache().Refresh(ctx, fetchToolviewsTools)
		} else {
			names, err = cachedToolviewsNames(ctx)
		}
		if err != nil {
			return err
		}
		out.Print(cmd, names)
		return nil
	}}
	command.Flags().BoolVar(&refresh, "refresh", false, "Fetch a fresh tool list instead of using the disk cache")
	command.Flags().DurationVar(&timeout, "timeout", 20*time.Second, "HTTP request timeout")
	out.AddFlagsWithOpts(command, output.WithDefaultTTY(output.TableType), output.WithDefaultPipe(output.JSONType), output.WithFilterFlagDisabled())
	return command
}
