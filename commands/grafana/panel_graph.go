package grafana

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
	"time"

	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

type grafanaFrame struct {
	Schema struct {
		Name   string         `json:"name"`
		Fields []grafanaField `json:"fields"`
	} `json:"schema"`
	Data struct {
		Values [][]json.RawMessage `json:"values"`
	} `json:"data"`
}

type grafanaField struct {
	Name   string            `json:"name"`
	Type   string            `json:"type"`
	Labels map[string]string `json:"labels"`
	Config struct {
		DisplayNameFromDS string `json:"displayNameFromDS"`
	} `json:"config"`
}

func printPanelGraphs(writer io.Writer, title string, data json.RawMessage, width int) error {
	var result struct {
		Results map[string]struct {
			Frames []grafanaFrame `json:"frames"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("decode Grafana data frames: %w", err)
	}
	var graphs []output.Graph
	hasSamples := false
	for _, refID := range slices.Sorted(maps.Keys(result.Results)) {
		for _, frame := range result.Results[refID].Frames {
			frameGraphs, err := graphFrame(title, refID, frame, width)
			if err != nil {
				return fmt.Errorf("graph query %s: %w", refID, err)
			}
			graphs = append(graphs, frameGraphs...)
			for _, column := range frame.Data.Values {
				hasSamples = hasSamples || len(column) > 0
			}
		}
	}
	if len(graphs) == 0 {
		if hasSamples {
			return fmt.Errorf("the panel returned no numeric time series; use --output json to inspect the data")
		}
		return (output.Graph{Title: title, Width: width}).Print(writer)
	}
	for i, graph := range graphs {
		if i > 0 {
			if _, err := fmt.Fprintln(writer); err != nil {
				return err
			}
		}
		if err := graph.Print(writer); err != nil {
			return err
		}
	}
	return nil
}

func graphFrame(title, refID string, frame grafanaFrame, width int) ([]output.Graph, error) {
	var timeValues []json.RawMessage
	for i, field := range frame.Schema.Fields {
		if field.Type == "time" && i < len(frame.Data.Values) {
			timeValues = frame.Data.Values[i]
			break
		}
	}
	if timeValues == nil {
		return nil, nil
	}

	var graphs []output.Graph
	for i, field := range frame.Schema.Fields {
		if field.Type != "number" || i >= len(frame.Data.Values) {
			continue
		}
		values := frame.Data.Values[i]
		if len(values) != len(timeValues) {
			return nil, fmt.Errorf("series %q has %d values for %d timestamps", field.Name, len(values), len(timeValues))
		}
		graph := output.Graph{
			Title: strings.Join([]string{title, refID, graphSeriesName(frame, field)}, " · "),
			Width: width, TimeFormat: "01-02 15:04Z",
		}
		for row, rawValue := range values {
			var value *float64
			if err := json.Unmarshal(rawValue, &value); err != nil {
				return nil, fmt.Errorf("decode numeric value in field %q: %w", field.Name, err)
			}
			if value == nil || string(timeValues[row]) == "null" {
				continue
			}
			var millis int64
			if err := json.Unmarshal(timeValues[row], &millis); err != nil {
				return nil, fmt.Errorf("decode sample timestamp: %w", err)
			}
			graph.Points = append(graph.Points, output.GraphPoint{Time: time.UnixMilli(millis).UTC(), Value: *value})
		}
		graphs = append(graphs, graph)
	}
	return graphs, nil
}

func graphSeriesName(frame grafanaFrame, field grafanaField) string {
	if field.Config.DisplayNameFromDS != "" {
		return field.Config.DisplayNameFromDS
	}
	if len(field.Labels) != 0 {
		var labels []string
		for _, label := range slices.Sorted(maps.Keys(field.Labels)) {
			labels = append(labels, label+"="+field.Labels[label])
		}
		return strings.Join(labels, ", ")
	}
	if frame.Schema.Name != "" {
		return frame.Schema.Name
	}
	return field.Name
}
