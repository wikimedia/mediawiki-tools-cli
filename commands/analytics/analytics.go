package analytics

import "github.com/spf13/cobra"

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "analytics", GroupID: "service", Short: "Query the Wikimedia Analytics API"}
	cmd.AddCommand(NewPageviewsCmd())
	for _, def := range []metricDefinition{
		{name: "top-pages", short: "List the most-viewed pages", example: "top-pages --project en.wikipedia.org --date 20250101", ranking: true, columns: []metricColumn{{"Rank", "rank"}, {"Article", "article"}, {"Views", "views"}}},
		{name: "edits", short: "Get edit counts for a page or project", example: "edits 'Albert Einstein' --start 20250101 --end 20250131", columns: []metricColumn{{"Date", "timestamp"}, {"Edits", "edits"}}},
		{name: "top-edited", short: "List the most-edited pages", example: "top-edited --project en.wikipedia.org --month 202501", ranking: true, columns: []metricColumn{{"Rank", "rank"}, {"Page", "page_title"}, {"Edits", "edits"}}},
		{name: "editors", short: "Get editor counts grouped by activity", example: "editors --activity-level 5..24-edits --start 20250101 --end 20250131", columns: []metricColumn{{"Date", "timestamp"}, {"Editors", "editors"}}},
		{name: "new-pages", short: "Get the number of newly created pages", example: "new-pages --project en.wikipedia.org --page-type content", columns: []metricColumn{{"Date", "timestamp"}, {"New pages", "new_pages"}}},
		{name: "devices", short: "Get estimated unique device counts", example: "devices --project en.wikipedia.org --granularity monthly --start 20250101 --end 20250201", columns: []metricColumn{{"Date", "timestamp"}, {"Devices", "devices"}}},
	} {
		cmd.AddCommand(newMetricCmd(def))
	}
	media := newMetricCmd(metricDefinition{name: "mediarequests", short: "Get media file request counts", example: "mediarequests --media-type image --start 20250101 --end 20250131", columns: []metricColumn{{"Date", "timestamp"}, {"Requests", "requests"}}})
	media.AddCommand(newMetricCmd(metricDefinition{name: "top", short: "List the most-requested media files", example: "top --date 20250101 --limit 10", ranking: true, columns: []metricColumn{{"Rank", "rank"}, {"File", "file_path"}, {"Requests", "requests"}}}))
	cmd.AddCommand(media)
	return cmd
}
