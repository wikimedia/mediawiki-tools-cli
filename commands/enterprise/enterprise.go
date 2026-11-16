package enterprise

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

func NewCmd() *cobra.Command {
	var profile string
	cmd := &cobra.Command{
		Use: "enterprise", GroupID: "service", Short: "Fetch Wikimedia Enterprise articles, datasets and changes",
		Long: "Access Wikimedia Enterprise with a saved Enterprise account. Fetch current articles, download snapshots and hourly batches, or follow realtime changes. Wikidata and Structured Contents endpoints are beta; realtime access requires an appropriate subscription.",
	}
	profiles.AddFlag(cmd, &profile)
	cmd.AddCommand(newAuthCmd(&profile), newAPICmd(&profile), newArticlesCmd(&profile), newSnapshotsCmd(&profile), newBatchesCmd(&profile), newStreamCmd(&profile), newWikidataCmd(&profile))
	for _, resource := range []string{"projects", "languages", "namespaces", "codes"} {
		parent := &cobra.Command{Use: resource, Short: "Inspect Enterprise " + resource}
		parent.AddCommand(newQueryCmd(&profile, "list", "List available "+resource, "list", 0, false, func(_ *cobra.Command, _ []string) (string, error) { return "/v2/" + resource, nil }))
		example := map[string]string{"projects": "enwiki", "languages": "en", "namespaces": "0", "codes": "wiki"}[resource]
		parent.AddCommand(newQueryCmd(&profile, "info <identifier>", "Show one "+strings.TrimSuffix(resource, "s"), "info "+example, 1, false, func(_ *cobra.Command, args []string) (string, error) {
			id, err := segment(args[0])
			return "/v2/" + resource + "/" + id, err
		}))
		cmd.AddCommand(parent)
	}
	return cmd
}

// segment protects identifier endpoints from accidental path traversal. Titles
// use PathEscape separately because slash is a valid character in page names.
func segment(value string) (string, error) {
	if value == "" || value == "." || value == ".." || strings.ContainsAny(value, "/\\") {
		return "", fmt.Errorf("invalid identifier %q", value)
	}
	return url.PathEscape(value), nil
}

type queryOptions struct {
	wiki, language  string
	fields, filters []string
	limit           int
}

func (q *queryOptions) flags(cmd *cobra.Command, limited bool) {
	cmd.Flags().StringVar(&q.wiki, "wiki", "", "Filter by wiki identifier, for example enwiki")
	cmd.Flags().StringVar(&q.language, "language", "", "Filter by language identifier, for example en")
	cmd.Flags().StringSliceVar(&q.fields, "fields", nil, "Request only these comma-separated API fields")
	cmd.Flags().StringArrayVarP(&q.filters, "filter", "f", nil, "Filter API results by field=value (repeatable; JSON scalars preserve types)")
	if limited {
		cmd.Flags().IntVar(&q.limit, "limit", 3, "Maximum exact matches returned by the API (1-10)")
	}
}

func (q queryOptions) body(limited bool) (map[string]any, error) {
	body := map[string]any{}
	filters := []map[string]any{}
	if q.wiki != "" {
		filters = append(filters, map[string]any{"field": "is_part_of.identifier", "value": q.wiki})
	}
	if q.language != "" {
		filters = append(filters, map[string]any{"field": "in_language.identifier", "value": q.language})
	}
	seen := map[string]bool{}
	for _, f := range filters {
		seen[f["field"].(string)] = true
	}
	for _, expression := range q.filters {
		field, raw, ok := strings.Cut(expression, "=")
		if !ok || strings.TrimSpace(field) == "" {
			return nil, fmt.Errorf("--filter requires field=value")
		}
		if seen[field] {
			return nil, fmt.Errorf("duplicate API filter for %s", field)
		}
		seen[field] = true
		var value any
		decoder := json.NewDecoder(strings.NewReader(raw))
		decoder.UseNumber()
		if decoder.Decode(&value) != nil || decoder.Decode(new(any)) != io.EOF {
			value = raw
		}
		switch value.(type) {
		case string, bool, json.Number:
		default:
			return nil, fmt.Errorf("API filter values must be strings, numbers or booleans")
		}
		filters = append(filters, map[string]any{"field": field, "value": value})
	}
	if len(filters) != 0 {
		body["filters"] = filters
	}
	if len(q.fields) != 0 {
		body["fields"] = q.fields
	}
	if limited {
		if q.limit < 1 || q.limit > 10 {
			return nil, fmt.Errorf("--limit must be between 1 and 10")
		}
		body["limit"] = q.limit
	}
	return body, nil
}

func newQueryCmd(profile *string, use, short, example string, argc int, limited bool, path func(*cobra.Command, []string) (string, error)) *cobra.Command {
	var opts queryOptions
	out := output.Output{}
	out.TableBinding = &output.TableBinding{Headings: []string{"Identifier", "Name", "Modified"}, RowExtractor: func(value any) []string {
		row, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		return []string{cell(row["identifier"]), cell(row["name"]), cell(row["date_modified"])}
	}}
	cmd := &cobra.Command{Use: use, Short: short, Example: example, Args: cobra.ExactArgs(argc)}
	opts.flags(cmd, limited)
	out.AddFlagsWithOpts(cmd, output.WithDefaultOutput(output.JSONType), output.WithFilterFlagDisabled())
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		path, err := path(cmd, args)
		if err != nil {
			return err
		}
		body, err := opts.body(limited)
		if err != nil {
			return err
		}
		if err := output.ValidateJSONFormat(out.Format); err != nil {
			return err
		}
		if out.Type != "json" && out.Type != "jq" && out.Type != "table" {
			return fmt.Errorf("--output must be json, jq or table")
		}
		c, err := client(*profile)
		if err != nil {
			return err
		}
		value, err := c.JSON(cmd.Context(), path, body)
		if err != nil {
			return err
		}
		if out.Type == "json" || out.Type == "jq" {
			return output.PrintJSONChecked(value, out.Format, out.Type == "jq", cmd.OutOrStdout())
		}
		out.Print(cmd, value)
		return nil
	}
	return cmd
}

func cell(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func newArticlesCmd(profile *string) *cobra.Command {
	var structured bool
	parent := &cobra.Command{Use: "articles", Short: "Look up current articles by exact title"}
	get := newQueryCmd(profile, "get <title>", "Fetch exact-title article matches", "get 'Albert Einstein' --wiki enwiki\nget Squirrel --wiki enwiki --structured", 1, true, func(_ *cobra.Command, args []string) (string, error) {
		if strings.TrimSpace(args[0]) == "" || args[0] == "." || args[0] == ".." {
			return "", fmt.Errorf("provide an article title")
		}
		resource := "articles"
		if structured {
			resource = "structured-contents"
		}
		return "/v2/" + resource + "/" + url.PathEscape(strings.ReplaceAll(args[0], " ", "_")), nil
	})
	get.Long = "Look up the current article by exact, case-sensitive title. Without --wiki or --language, the API can return matches from several projects. The default limit is 3 and the maximum is 10. --structured selects the beta Structured Contents endpoint. JSON output preserves the API's array of matches."
	get.Flags().BoolVar(&structured, "structured", false, "Fetch beta Structured Contents with parsed sections and infoboxes")
	parent.AddCommand(get)
	return parent
}

func newWikidataCmd(profile *string) *cobra.Command {
	parent := &cobra.Command{Use: "wikidata", Short: "Look up Wikidata items, properties and labels (beta)"}
	for _, resource := range []string{"items", "properties", "labels"} {
		group := &cobra.Command{Use: resource, Short: "Retrieve Wikidata " + resource + " (beta)"}
		example := "get Q42"
		if resource == "properties" {
			example = "get P31"
		}
		group.AddCommand(newQueryCmd(profile, "get <identifier>", "Fetch Wikidata "+resource+" (beta)", example, 1, true, func(_ *cobra.Command, args []string) (string, error) {
			id := args[0]
			prefix := "Q"
			if resource == "properties" {
				prefix = "P"
			}
			if resource == "labels" && strings.HasPrefix(id, "P") {
				prefix = "P"
			}
			if len(id) < 2 || !strings.HasPrefix(id, prefix) {
				return "", fmt.Errorf("expected a %s identifier", prefix)
			}
			if _, err := strconv.ParseUint(id[1:], 10, 64); err != nil {
				return "", fmt.Errorf("invalid Wikidata identifier")
			}
			return "/v2/wikidata/" + resource + "/" + id, nil
		}))
		parent.AddCommand(group)
	}
	return parent
}

func newSnapshotsCmd(profile *string) *cobra.Command {
	var structured, wikidata bool
	parent := &cobra.Command{Use: "snapshots", Short: "Inspect and download full project snapshots"}
	parent.PersistentFlags().BoolVar(&structured, "structured", false, "Use beta Structured Contents snapshots")
	parent.PersistentFlags().BoolVar(&wikidata, "wikidata", false, "Use beta Wikidata snapshots (items or properties)")
	base := func() (string, error) {
		if structured && wikidata {
			return "", fmt.Errorf("--structured and --wikidata cannot be combined")
		}
		path := "/v2/snapshots"
		if structured {
			path += "/structured-contents"
		}
		if wikidata {
			path += "/wikidata"
		}
		return path, nil
	}
	path := func(_ *cobra.Command, args []string) (string, error) {
		p, err := base()
		if err != nil {
			return "", err
		}
		if len(args) > 0 {
			id, err := datasetID(args[0], wikidata)
			if err != nil {
				return "", err
			}
			p += "/" + id
		}
		return p, nil
	}
	parent.AddCommand(newQueryCmd(profile, "list", "List available snapshots", "list --wiki enwiki", 0, false, path))
	parent.AddCommand(newQueryCmd(profile, "info <snapshot>", "Inspect a snapshot", "info enwiki_namespace_0", 1, false, path))
	parent.AddCommand(newDownloadCmd(profile, "download <snapshot>", "Download a snapshot tarball", "download enwiki_namespace_0 --file enwiki.tar.gz", 1, path))
	chunks := &cobra.Command{Use: "chunks", Short: "Inspect and download individual snapshot chunks"}
	chunkPath := func(_ *cobra.Command, args []string) (string, error) {
		if structured {
			return "", fmt.Errorf("Structured Contents snapshots do not expose chunks")
		}
		p, err := path(nil, args[:1])
		if err != nil {
			return "", err
		}
		p += "/chunks"
		if len(args) == 2 {
			id, err := segment(args[1])
			if err != nil {
				return "", err
			}
			p += "/" + id
		}
		return p, nil
	}
	chunks.AddCommand(newQueryCmd(profile, "list <snapshot>", "List snapshot chunks", "list enwiki_namespace_0", 1, false, chunkPath))
	chunks.AddCommand(newQueryCmd(profile, "info <snapshot> <chunk>", "Inspect a snapshot chunk", "info enwiki_namespace_0 CHUNK", 2, false, chunkPath))
	chunks.AddCommand(newDownloadCmd(profile, "download <snapshot> <chunk>", "Download one snapshot chunk", "download enwiki_namespace_0 CHUNK --file chunk.tar.gz", 2, chunkPath))
	parent.AddCommand(chunks)
	return parent
}

func datasetID(id string, wikidata bool) (string, error) {
	if wikidata {
		id = strings.TrimPrefix(id, "wikidata_")
		if id != "items" && id != "properties" {
			return "", fmt.Errorf("Wikidata dataset must be items or properties")
		}
	}
	return segment(id)
}

func newBatchesCmd(profile *string) *cobra.Command {
	var date string
	var hour int
	var wikidata bool
	parent := &cobra.Command{Use: "batches", Short: "Inspect and download hourly change batches (paid access)", Long: "Download hourly change archives. Dates and hours are UTC. Availability is limited by server retention; list the requested hour before downloading."}
	parent.PersistentFlags().StringVar(&date, "date", "", "Batch date in UTC (YYYY-MM-DD; required)")
	parent.PersistentFlags().IntVar(&hour, "hour", -1, "Batch hour in UTC (0-23; required)")
	parent.PersistentFlags().BoolVar(&wikidata, "wikidata", false, "Use beta Wikidata batches (items or properties)")
	path := func(_ *cobra.Command, args []string) (string, error) {
		if _, err := time.Parse("2006-01-02", date); err != nil {
			return "", fmt.Errorf("--date must be YYYY-MM-DD in UTC")
		}
		if hour < 0 || hour > 23 {
			return "", fmt.Errorf("--hour must be 0-23 in UTC")
		}
		p := fmt.Sprintf("/v2/batches/%s/%02d", date, hour)
		if wikidata {
			p += "/wikidata"
		}
		if len(args) > 0 {
			id, err := datasetID(args[0], wikidata)
			if err != nil {
				return "", err
			}
			p += "/" + id
		}
		return p, nil
	}
	parent.AddCommand(newQueryCmd(profile, "list", "List batches for one UTC hour", "list --date 2026-11-04 --hour 12", 0, false, path))
	parent.AddCommand(newQueryCmd(profile, "info <identifier>", "Inspect one hourly batch", "info enwiki_namespace_0 --date 2026-11-04 --hour 12", 1, false, path))
	parent.AddCommand(newDownloadCmd(profile, "download <identifier>", "Download an hourly batch tarball", "download enwiki_namespace_0 --date 2026-11-04 --hour 12 --file changes.tar.gz", 1, path))
	return parent
}
