package events

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

type streamInfo struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"schema,omitempty"`
	Example     json.RawMessage `json:"example,omitempty"`
}

type streamOperation struct {
	Summary     string `json:"summary"`
	Description string `json:"description"`
	Responses   map[string]struct {
		Content map[string]struct {
			Schema  json.RawMessage `json:"schema"`
			Example json.RawMessage `json:"example"`
		} `json:"content"`
	} `json:"responses"`
}

func fetchStreams(ctx context.Context) ([]streamInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, eventStreamsURL+"/?spec", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", eventStreamsUserAgent)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("EventStreams discovery returned %s", resp.Status)
	}
	var spec struct {
		Paths map[string]struct {
			Get streamOperation `json:"get"`
		} `json:"paths"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(&spec); err != nil {
		return nil, fmt.Errorf("decode EventStreams specification: %w", err)
	}
	streams := make([]streamInfo, 0, len(spec.Paths))
	for path, route := range spec.Paths {
		name, ok := strings.CutPrefix(path, "/v2/stream/")
		if !ok || name == "" || strings.ContainsAny(name, "{}/") {
			continue
		}
		description := route.Get.Description
		if description == "" {
			description = route.Get.Summary
		}
		content := route.Get.Responses["200"].Content["application/json"]
		streams = append(streams, streamInfo{Name: name, Description: strings.TrimSpace(description), Schema: content.Schema, Example: content.Example})
	}
	if len(streams) == 0 {
		return nil, fmt.Errorf("EventStreams specification contains no public streams")
	}
	sort.Slice(streams, func(i, j int) bool { return streams[i].Name < streams[j].Name })
	return streams, nil
}

func NewListCmd() *cobra.Command {
	type summary struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	out := output.Output{}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available public Wikimedia event streams",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			streams, err := fetchStreams(cmd.Context())
			if err != nil {
				return err
			}
			items := make([]summary, 0, len(streams))
			for _, stream := range streams {
				description, _, _ := strings.Cut(stream.Description, "\n")
				items = append(items, summary{Name: stream.Name, Description: description})
			}
			out.Print(cmd, items)
			return nil
		},
	}
	out.AddFlagsWithOpts(cmd,
		output.WithDefaultTTY(output.TableType),
		output.WithDefaultPipe(output.JSONType),
		output.WithFilterFlagDisabled(),
		output.WithTableBinding(&output.TableBinding{
			Headings: []string{"Name", "Description"},
			ProcessObjects: func(objects interface{}, table *output.Table) {
				for _, item := range objects.([]summary) {
					table.AddRowS(item.Name, item.Description)
				}
			},
		}),
	)
	return cmd
}

func NewInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "info STREAM",
		Short:   "Show an event stream's description, schema and example",
		Example: "info recentchange",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			streams, err := fetchStreams(cmd.Context())
			if err != nil {
				return err
			}
			for _, stream := range streams {
				if stream.Name != args[0] {
					continue
				}
				data, err := json.Marshal(stream)
				if err != nil {
					return err
				}
				return output.PrintJSONEvent(cmd.OutOrStdout(), data)
			}
			return fmt.Errorf("unknown public stream %q; use mw events list to see available streams", args[0])
		},
	}
}
