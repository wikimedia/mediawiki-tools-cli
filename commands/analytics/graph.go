package analytics

import (
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

func validateGraphWidth(width int) error {
	if width != 0 && (width < 24 || width > 1000) {
		return fmt.Errorf("--graph-width must be 0 (automatic), or between 24 and 1000")
	}
	return nil
}

func metricGraphTime(value string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, "2006010215", "20060102", "2006-01-02"} {
		if date, err := time.Parse(layout, value); err == nil {
			return date, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot graph invalid observation date %q", value)
}

func printMetricGraph(cmd *cobra.Command, rows []map[string]interface{}, def metricDefinition, opts metricOptions) error {
	valueColumn := def.columns[len(def.columns)-1]
	subject := opts.project
	if subject == "" {
		subject = opts.referer + " / " + opts.mediaType
	}
	if def.name == "edits" && len(cmd.Flags().Args()) > 0 {
		subject = cmd.Flags().Args()[0] + " (" + subject + ")"
	}
	graph := output.Graph{Title: valueColumn.heading + " — " + subject, Bars: def.ranking, Width: opts.graphWidth}
	for _, row := range rows {
		value, err := strconv.ParseFloat(fmt.Sprint(row[valueColumn.key]), 64)
		if err != nil {
			return fmt.Errorf("cannot graph non-numeric %s value", valueColumn.key)
		}
		point := output.GraphPoint{Value: value}
		if def.ranking {
			point.Label = metricCell(row, def.columns[1].key, false)
		} else {
			point.Time, err = metricGraphTime(fmt.Sprint(row["timestamp"]))
			if err != nil {
				return err
			}
		}
		graph.Points = append(graph.Points, point)
	}
	return graph.Print(cmd.OutOrStdout())
}
