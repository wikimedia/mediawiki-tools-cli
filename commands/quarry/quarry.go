package quarry

import (
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
)

const BaseURL = "https://quarry.wmcloud.org"

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "quarry", GroupID: "service", Short: "Download public Wikimedia SQL query results"}
	cmd.AddCommand(newResultsCmd())
	return cmd
}

func newResultsCmd() *cobra.Command {
	var runID uint64
	var resultSet uint
	var format string
	cmd := &cobra.Command{
		Use:   "results [query-id]",
		Short: "Download results from a public Quarry query or run",
		Long:  "Download a result set from the latest run of a public Quarry query, or use --run with an exact run ID. Results are written to standard output in Quarry's original format.",
		Example: cobrautil.NormalizeExample(`
results 541
results 541 --format csv > results.csv
results --run 1534 --result-set 0 --format json
`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch format {
			case "json", "csv", "tsv", "wikitable":
			default:
				return fmt.Errorf("unsupported Quarry format %q, choose json, csv, tsv or wikitable", format)
			}

			var path string
			if cmd.Flags().Changed("run") {
				if len(args) != 0 || runID == 0 {
					return fmt.Errorf("provide a positive run ID with --run in place of the query ID")
				}
				path = fmt.Sprintf("/run/%d/output/%d/%s", runID, resultSet, format)
			} else {
				if len(args) == 0 {
					return fmt.Errorf("provide a query ID or use --run with a run ID")
				}
				queryID, err := strconv.ParseUint(args[0], 10, 64)
				if err != nil || queryID == 0 {
					return fmt.Errorf("invalid Quarry query ID %q, expected a positive integer", args[0])
				}
				path = fmt.Sprintf("/query/%d/result/latest/%d/%s", queryID, resultSet, format)
			}

			req, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, BaseURL+path, nil)
			if err != nil {
				return err
			}
			resp, err := cli.NewHTTPClient().Do(req)
			if err != nil {
				return fmt.Errorf("download Quarry results: %w", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("download Quarry results from %s: HTTP %d %s", path, resp.StatusCode, http.StatusText(resp.StatusCode))
			}
			_, err = io.Copy(cmd.OutOrStdout(), resp.Body)
			return err
		},
	}
	cmd.Flags().Uint64Var(&runID, "run", 0, "Exact Quarry run ID")
	cmd.Flags().UintVar(&resultSet, "result-set", 0, "Zero-based result set index")
	cmd.Flags().StringVar(&format, "format", "json", "Result format (json, csv, tsv, wikitable)")
	return commandmeta.Set(cmd, commandmeta.Hints{
		ToolAnnotations: commandmeta.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: true},
		DataAccess:      []commandmeta.DataAccess{commandmeta.DataPublic},
	})
}
