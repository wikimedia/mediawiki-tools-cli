package grafana

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

type dashboardModel struct {
	UID        string           `json:"uid"`
	Panels     []dashboardPanel `json:"panels"`
	Templating struct {
		List []struct {
			Name    string `json:"name"`
			Current struct {
				Value json.RawMessage `json:"value"`
			} `json:"current"`
		} `json:"list"`
	} `json:"templating"`
}

type dashboardPanel struct {
	ID         int              `json:"id"`
	Title      string           `json:"title"`
	Type       string           `json:"type"`
	Datasource json.RawMessage  `json:"datasource"`
	Targets    []panelTarget    `json:"targets"`
	Panels     []dashboardPanel `json:"panels"`
}

type panelTarget struct {
	RefID      string          `json:"refId"`
	Datasource json.RawMessage `json:"datasource"`
	Expr       json.RawMessage `json:"expr"`
	Hidden     bool            `json:"hide"`
	Instant    bool            `json:"instant"`
	Range      *bool           `json:"range"`
	MinStep    string          `json:"interval"`
	Format     string          `json:"format"`
}

type panelSource struct {
	UID  string `json:"uid,omitempty"`
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
}

func sourceReference(raw json.RawMessage) panelSource {
	var source panelSource
	if json.Unmarshal(raw, &source) != nil {
		_ = json.Unmarshal(raw, &source.Name)
	}
	return source
}

func (t panelTarget) source(panel dashboardPanel) panelSource {
	if source := sourceReference(t.Datasource); source.UID != "" || source.Name != "" {
		return source
	}
	return sourceReference(panel.Datasource)
}

func (t panelTarget) expression() string {
	var expr string
	_ = json.Unmarshal(t.Expr, &expr)
	return expr
}

func loadDashboard(ctx context.Context, baseURL, selector string) (dashboardModel, error) {
	var dashboard dashboardModel
	data, err := readDashboard(ctx, baseURL, selector)
	if err != nil {
		return dashboard, err
	}
	if err := json.Unmarshal(data, &dashboard); err != nil {
		return dashboard, fmt.Errorf("decode Grafana dashboard panels: %w", err)
	}
	return dashboard, nil
}

func flattenPanels(panels []dashboardPanel) []dashboardPanel {
	var result []dashboardPanel
	for _, panel := range panels {
		if panel.Type != "row" {
			result = append(result, panel)
		}
		result = append(result, flattenPanels(panel.Panels)...)
	}
	return result
}

type panelSummary struct {
	ID      int          `json:"id"`
	Title   string       `json:"title"`
	Queries []queryGuide `json:"queries"`
}

type queryGuide struct {
	RefID      string      `json:"ref_id"`
	Datasource panelSource `json:"datasource"`
	Expression string      `json:"expression,omitempty"`
	Hidden     bool        `json:"hidden,omitempty"`
}

func newDashboardPanelsCmd(baseURL string) *cobra.Command {
	var panelSelector string
	var timeout time.Duration
	out := output.Output{}
	cmd := &cobra.Command{
		Use: "panels <dashboard>", Short: "List dashboard panels or inspect one panel's queries", Args: cobra.ExactArgs(1),
		Long:    "Use a dashboard title or UID. List panels with their IDs and data sources, or pass a panel title or ID with --panel to see its queries.",
		Example: "panels 'MediaWiki LinksUpdate'\npanels 'MediaWiki LinksUpdate' --panel 'RefreshLinksJob good outcome'",
		RunE: func(cmd *cobra.Command, args []string) error {
			selected := cmd.Flags().Changed("panel")
			if out.Type == string(output.TableType) {
				if out.Format != "" {
					return fmt.Errorf("--format requires JSON or jq output")
				}
			} else if err := validateJSONOutput(out); err != nil {
				return err
			}
			if timeout <= 0 {
				return fmt.Errorf("--timeout must be positive")
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()
			dashboard, err := loadDashboard(ctx, baseURL, args[0])
			if err != nil {
				return err
			}

			panels := flattenPanels(dashboard.Panels)
			if selected {
				panel, err := selectPanel(panels, panelSelector, args[0])
				if err != nil {
					return err
				}
				panels = []dashboardPanel{panel}
			}
			summaries := make([]panelSummary, 0, len(panels))
			for _, panel := range panels {
				summary := panelSummary{ID: panel.ID, Title: panel.Title, Queries: []queryGuide{}}
				for _, target := range panel.Targets {
					summary.Queries = append(summary.Queries, queryGuide{
						RefID: target.RefID, Datasource: target.source(panel), Expression: target.expression(), Hidden: target.Hidden,
					})
				}
				summaries = append(summaries, summary)
			}

			if out.Type != string(output.TableType) {
				if selected {
					return output.PrintJSONChecked(summaries[0], out.Format, out.Type == string(output.JQType), cmd.OutOrStdout())
				}
				return output.PrintJSONChecked(summaries, out.Format, out.Type == string(output.JQType), cmd.OutOrStdout())
			}
			if selected {
				return printPanel(cmd.OutOrStdout(), summaries[0])
			}
			table := &output.Table{}
			table.AddHeadingsS("ID", "Panel", "Queries", "Data sources")
			for _, panel := range summaries {
				var sources []string
				for _, target := range panel.Queries {
					name := sourceLabel(target.Datasource)
					if !slices.Contains(sources, name) {
						sources = append(sources, name)
					}
				}
				table.AddRow(panel.ID, panel.Title, len(panel.Queries), strings.Join(sources, ", "))
			}
			table.Print(cmd.OutOrStdout())
			return nil
		},
	}
	cmd.Flags().StringVar(&panelSelector, "panel", "", "Show queries for this panel title or ID")
	cmd.Flags().DurationVar(&timeout, "timeout", 20*time.Second, "HTTP request timeout")
	out.AddFlagsWithOpts(cmd, output.WithTableBinding(&output.TableBinding{}), output.WithDefaultOutput(output.TableType), output.WithFilterFlagDisabled())
	return cmd
}

func sourceLabel(source panelSource) string {
	if source.UID != "" {
		return source.UID
	}
	if source.Name != "" {
		return source.Name
	}
	return "dashboard default"
}

func printPanel(writer io.Writer, panel panelSummary) error {
	var text strings.Builder
	fmt.Fprintf(&text, "%s (panel %d)\n", panel.Title, panel.ID)
	if len(panel.Queries) == 0 {
		fmt.Fprintln(&text, "This panel has no saved queries.")
	}
	for _, target := range panel.Queries {
		hidden := ""
		if target.Hidden {
			hidden = " (hidden)"
		}
		fmt.Fprintf(&text, "\nQuery %s%s · Data source %s\n", target.RefID, hidden, sourceLabel(target.Datasource))
		if target.Expression != "" {
			fmt.Fprintln(&text, target.Expression)
		} else {
			fmt.Fprintln(&text, "Use the panel's Query inspector to view its plugin-specific request.")
		}
	}
	_, err := io.WriteString(writer, text.String())
	return err
}
