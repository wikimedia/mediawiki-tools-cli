package wikidata

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func NewQueryCmd() *cobra.Command {
	var endpoint, format, file string
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:     "query [SPARQL]",
		Short:   "Run a SPARQL query against Wikidata Query Service",
		Long:    "Submit a SPARQL query to the public Wikidata Query Service and write its response to stdout. The public endpoint has a short execution deadline; keep queries selective and avoid repeated polling.",
		Example: "query 'SELECT ?item WHERE { ?item wdt:P31 wd:Q5 } LIMIT 10'\nquery --file people.rq --format csv\nquery --format tsv < query.rq",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var query string
			if file != "" {
				if len(args) != 0 {
					return fmt.Errorf("provide the query either as an argument or with --file")
				}
				b, err := os.ReadFile(file)
				if err != nil {
					return fmt.Errorf("read query file: %w", err)
				}
				query = string(b)
			} else if len(args) == 1 {
				query = args[0]
			} else if input, ok := cmd.InOrStdin().(*os.File); ok && !term.IsTerminal(int(input.Fd())) {
				b, err := io.ReadAll(io.LimitReader(cmd.InOrStdin(), 4*1024*1024+1))
				if err != nil {
					return err
				}
				if len(b) > 4*1024*1024 {
					return fmt.Errorf("query input exceeds 4 MiB")
				}
				query = string(b)
			}
			if strings.TrimSpace(query) == "" {
				return fmt.Errorf("provide a SPARQL query as an argument, with --file, or on stdin")
			}
			if timeout <= 0 || timeout > time.Minute {
				return fmt.Errorf("--timeout must be greater than zero and no more than 1m (the public endpoint deadline)")
			}
			accept, err := acceptFor(format)
			if err != nil {
				return err
			}
			return submit(cmd.Context(), endpoint, query, accept, timeout, cmd.OutOrStdout())
		},
	}
	cmd.Flags().StringVar(&endpoint, "endpoint", "https://query.wikidata.org/sparql", "SPARQL endpoint URL")
	cmd.Flags().StringVar(&format, "format", "json", "Response format: json, csv, tsv, or xml")
	cmd.Flags().StringVarP(&file, "file", "f", "", "Read the SPARQL query from a file")
	cmd.Flags().DurationVar(&timeout, "timeout", 55*time.Second, "Request timeout (maximum 1m)")
	return cmd
}

func acceptFor(format string) (string, error) {
	switch format {
	case "json":
		return "application/sparql-results+json", nil
	case "csv":
		return "text/csv", nil
	case "tsv":
		return "text/tab-separated-values", nil
	case "xml":
		return "application/sparql-results+xml", nil
	default:
		return "", fmt.Errorf("--format must be json, csv, tsv, or xml")
	}
}

func submit(parent context.Context, endpoint, query, accept string, timeout time.Duration, out io.Writer) error {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return fmt.Errorf("--endpoint must be an HTTPS URL")
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	form := url.Values{"query": {query}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", accept)
	req.Header.Set("User-Agent", "mwcli WikidataQuery/1.0 (https://www.mediawiki.org/wiki/CLI)")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		message := fmt.Sprintf("Wikidata Query Service returned %s", resp.Status)
		if retry := resp.Header.Get("Retry-After"); retry != "" {
			message += " (Retry-After: " + retry + ")"
		}
		if detail := strings.TrimSpace(string(body)); detail != "" {
			message += ": " + detail
		}
		return fmt.Errorf("%s", message)
	}
	_, err = io.Copy(out, resp.Body)
	return err
}
