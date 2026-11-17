package dumps

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
)

func newMirrorsCmd(opts *options, ask func(*cobra.Command, string) error) *cobra.Command {
	cmd := &cobra.Command{
		Use: "mirrors", Short: "Explore known mirrors and optionally compare download speeds",
		Long: "List known HTTPS mirrors or explicitly test a selected file. Mirrors can lag behind Wikimedia or carry only some datasets. Names come from https://meta.wikimedia.org/wiki/Mirroring_Wikimedia_project_XML_dumps; listing does not contact them.",
	}
	var listFormat string
	list := &cobra.Command{
		Use: "list", Short: "List known mirror URLs and coverage notes",
		Long:    "Print the built-in HTTPS mirror catalog without contacting any server. Coverage and freshness are not guaranteed; use test with approval to compare a particular file.",
		Example: cobrautil.NormalizeExample("list\nlist --output json"),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			rows := make([][]string, len(mirrors))
			for i, m := range mirrors {
				rows[i] = []string{m.Name, m.URL, m.Notes}
			}
			return printRows(cmd, listFormat, mirrors, []string{"Name", "URL", "Notes"}, rows)
		},
	}
	list.Flags().StringVar(&listFormat, "output", "table", "Output format: table or json")
	cmd.AddCommand(list, newMirrorTestCmd(opts, ask))
	return cmd
}

type speedResult struct {
	Mirror         string  `json:"mirror"`
	URL            string  `json:"url"`
	Bytes          int64   `json:"bytes"`
	Seconds        float64 `json:"seconds"`
	BytesPerSecond float64 `json:"bytes_per_second"`
	Error          string  `json:"error,omitempty"`
}

func newMirrorTestCmd(opts *options, ask func(*cobra.Command, string) error) *cobra.Command {
	var yes bool
	var format string
	var sampleMiB int64
	var sampleTimeout time.Duration
	cmd := &cobra.Command{
		Use: "test <file-path> [mirror...]", Short: "Compare bounded sample downloads after approval",
		Long: `Test the same file at selected known mirrors or HTTPS base URLs, serially.
Without mirror arguments, test the built-in catalog. By default each sample
reads at most 32 MiB or runs for at most 10 seconds, whichever comes first,
including connection setup. --sample-mib and --sample-timeout change these
limits. Servers which ignore Range are still read only up to the sample limit.
No file is saved. A time-limited sample with received data is a successful
measurement; a timeout before any data is an error.

The command first shows the target hosts and asks permission, including for
Wikimedia itself. --yes explicitly approves these speed-test requests only.
Redirects to other non-Wikimedia hosts still require separate approval.
Tests do not approve later downloads or automatically choose a source.
Results are approximate and reflect this file and moment, not a guarantee of
mirror completeness or large-file speed. Failed samples include an error;
if every sample fails the command returns a failure.`,
		Example:           cobrautil.NormalizeExample("test wikidata/latest-lexemes.json.bz2 wikimedia umu\ntest other/pageviews/2026/2026-11/pageviews-20261101-000000.gz wikimedia your umu --sample-mib 64 --sample-timeout 15s"),
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateFormat(format); err != nil {
				return err
			}
			// Limit multiplication to avoid overflowing the Range and reader bounds.
			if sampleMiB < 1 || sampleMiB > (1<<63-1)/(1<<20) {
				return fmt.Errorf("--sample-mib must be a positive whole number that fits in bytes")
			}
			if sampleTimeout <= 0 {
				return fmt.Errorf("--sample-timeout must be greater than zero")
			}
			relative, err := dumpPath(args[0])
			if err != nil {
				return err
			}
			if relative == "" || strings.HasSuffix(relative, "/") {
				return fmt.Errorf("speed tests require one file path")
			}
			names := args[1:]
			if len(names) == 0 {
				for _, m := range mirrors {
					names = append(names, m.Name)
				}
			}
			testOpts := *opts
			testOpts.approvedHosts = append([]string(nil), opts.approvedHosts...)
			var results []speedResult
			for _, name := range names {
				base, err := mirrorURL(name)
				if err != nil {
					return err
				}
				target, targetErr := mirrorTarget(base, relative)
				if targetErr != nil {
					results = append(results, speedResult{Mirror: name, Error: targetErr.Error()})
					continue
				}
				results = append(results, speedResult{Mirror: name, URL: target.String()})
				testOpts.approvedHosts = append(testOpts.approvedHosts, target.Host)
				if _, err := fmt.Fprintln(cmd.ErrOrStderr(), "Speed-test target:", target.String()); err != nil {
					return err
				}
			}
			if !yes {
				targets := 0
				for _, result := range results {
					if result.URL != "" {
						targets++
					}
				}
				if targets > 0 {
					if err := ask(cmd, fmt.Sprintf("Run %d sample downloads (up to %d MiB or %s each, whichever comes first)? Use --yes for explicit noninteractive approval", targets, sampleMiB, sampleTimeout)); err != nil {
						return err
					}
				}
			}
			client := testOpts.client(cmd, ask)
			defer client.http.CloseIdleConnections()
			return runCancelable(cmd, func(ctx context.Context) error {
				success := false
				for i := range results {
					if results[i].Error != "" {
						continue
					}
					results[i] = client.speed(ctx, results[i], sampleMiB*(1<<20), sampleTimeout)
					success = success || results[i].Error == ""
					if ctx.Err() != nil {
						return ctx.Err()
					}
				}
				rows := make([][]string, len(results))
				for i, r := range results {
					rows[i] = []string{r.Mirror, fmt.Sprintf("%.2f", r.BytesPerSecond/(1<<20)), fmt.Sprint(r.Bytes), r.Error}
				}
				if err := printRows(cmd, format, results, []string{"Mirror", "MiB/s", "Sample bytes", "Error"}, rows); err != nil {
					return err
				}
				if !success {
					return fmt.Errorf("all mirror speed tests failed")
				}
				return nil
			})
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Explicitly approve only this run's bounded speed tests")
	cmd.Flags().StringVar(&format, "output", "table", "Output format: table or json")
	cmd.Flags().Int64Var(&sampleMiB, "sample-mib", 32, "Maximum MiB read per mirror during a speed test")
	cmd.Flags().DurationVar(&sampleTimeout, "sample-timeout", 10*time.Second, "Maximum time per mirror, including connection setup")
	return cmd
}

func (c *dumpClient) speed(parent context.Context, result speedResult, sampleSize int64, timeout time.Duration) speedResult {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	start := time.Now()
	target, err := url.Parse(result.URL)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	headers := make(http.Header)
	headers.Set("Range", fmt.Sprintf("bytes=0-%d", sampleSize-1))
	resp, err := c.request(ctx, http.MethodGet, target, headers)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		result.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		return result
	}
	result.Bytes, err = io.Copy(io.Discard, io.LimitReader(resp.Body, sampleSize))
	result.Seconds = time.Since(start).Seconds()
	if parent.Err() != nil {
		result.Error = parent.Err().Error()
	} else if err != nil && !(result.Bytes > 0 && errors.Is(err, context.DeadlineExceeded) && errors.Is(ctx.Err(), context.DeadlineExceeded)) {
		result.Error = err.Error()
	}
	if result.Bytes == 0 && result.Error == "" {
		result.Error = "empty sample"
	}
	if result.Seconds > 0 {
		result.BytesPerSecond = float64(result.Bytes) / result.Seconds
	}
	return result
}
