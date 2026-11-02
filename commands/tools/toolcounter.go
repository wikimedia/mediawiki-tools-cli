package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/restapi"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

type toolcounterStats struct {
	Tool     string `json:"tool"`
	Page     string `json:"page"`
	Views    int64  `json:"views"`
	Sessions int64  `json:"sessions"`
}

func fetchToolcounter(ctx context.Context, tool, page string) (toolcounterStats, error) {
	query := url.Values{"tool": {tool}, "page": {page}, "action": {"get"}}
	response, err := restapi.Request(ctx, http.MethodGet, "https://toolcounter.toolforge.org/", "api.php?"+query.Encode(), nil, nil)
	if err != nil {
		return toolcounterStats{}, err
	}
	defer response.Body.Close()
	if err := restapi.CheckStatus(response); err != nil {
		return toolcounterStats{}, err
	}
	var result struct {
		Tool     string `json:"tool"`
		Page     string `json:"page"`
		Views    *int64 `json:"views"`
		Sessions *int64 `json:"sessions"`
		Error    string `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return toolcounterStats{}, fmt.Errorf("decode Toolcounter response: %w", err)
	}
	if result.Error != "" {
		return toolcounterStats{}, fmt.Errorf("Toolcounter: %s", result.Error)
	}
	if result.Views == nil || result.Sessions == nil || *result.Views < 0 || *result.Sessions < 0 {
		return toolcounterStats{}, fmt.Errorf("Toolcounter response is missing valid views or sessions counts")
	}
	if result.Tool != tool || result.Page != page {
		return toolcounterStats{}, fmt.Errorf("Toolcounter returned counts for %q page %q instead of the requested tool and page", result.Tool, result.Page)
	}
	return toolcounterStats{Tool: result.Tool, Page: result.Page, Views: *result.Views, Sessions: *result.Sessions}, nil
}

func newToolcounterCmd() *cobra.Command {
	var page string
	var timeout time.Duration
	out := output.Output{TableBinding: &output.TableBinding{
		Headings: []string{"Tool", "Page", "Views", "Sessions"},
		ProcessObjects: func(objects interface{}, table *output.Table) {
			stats := objects.(toolcounterStats)
			table.AddRow(stats.Tool, stats.Page, stats.Views, stats.Sessions)
		},
	}}
	command := &cobra.Command{
		Use:   "toolcounter <tool>",
		Short: "Read page views and sessions from Toolcounter",
		Long: "Read the public Toolcounter counts for one tool and page without incrementing them. " +
			"The page is the identifier used in the site's embed code, not necessarily a URL; its default is index. " +
			"For the a Wikimedia CLI documentation site, use a path such as / or /manual/wiki/. " +
			"Views count page loads; sessions approximate visits per browser tab and are not unique people. " +
			"Counts cover all time. The documented API does not provide page discovery or historical queries. No login is required.",
		Example: "toolcounter a-wikimedia-cli --page /\ntoolcounter a-wikimedia-cli --page /manual/wiki/ --output json\ntoolcounter example --page index --output jq --format .views",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tool := args[0]
			if strings.TrimSpace(tool) == "" || strings.ContainsAny(tool, "/\\?# \t\r\n") || tool == "." || tool == ".." || strings.ContainsFunc(tool, unicode.IsControl) {
				return fmt.Errorf("tool must be a Toolforge tool name")
			}
			if strings.TrimSpace(page) == "" || strings.ContainsFunc(page, unicode.IsControl) {
				return fmt.Errorf("--page must be a nonempty identifier without control characters")
			}
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
			stats, err := fetchToolcounter(ctx, tool, page)
			if err != nil {
				return err
			}
			return out.Print(cmd, stats)
		},
	}
	command.Flags().StringVar(&page, "page", "index", "Exact page identifier used by the Toolcounter embed")
	command.Flags().DurationVar(&timeout, "timeout", 20*time.Second, "HTTP request timeout")
	out.AddFlagsWithOpts(command, output.WithDefaultTTY(output.TableType), output.WithDefaultPipe(output.JSONType), output.WithFilterFlagDisabled())
	return command
}
