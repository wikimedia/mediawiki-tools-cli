// Package api implements direct REST requests to Wikimedia services.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/restapi"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

func NewCmd() *cobra.Command {
	var baseURL, method, input string
	var headers, fields []string
	var raw, include bool
	var timeout time.Duration
	out := output.Output{}
	cmd := &cobra.Command{
		Use: "api <endpoint-or-url>", GroupID: "service",
		Short:   "Make a direct REST request to a Wikimedia API",
		Long:    "Make a REST request relative to https://wikimedia.org/api/rest_v1/, or supply an absolute HTTP(S) URL. JSON is indented and coloured in a terminal and compact when piped. Other response bodies are printed unchanged. Requests return a failing exit status for HTTP errors. No credentials are added automatically.\n\nFields are strings: they form a JSON body and select POST by default, or query parameters with --method GET. Use --input FILE (or - for stdin) for a custom body, and --header for authentication or content types.",
		Example: "api 'metrics/pageviews/top/en.wikipedia.org/all-access/2025/01/01'\napi 'https://en.wikipedia.org/w/rest.php/v1/search/page' -X GET -f q=Earth\napi 'metrics/pageviews/top/en.wikipedia.org/all-access/2025/01/01' --output jq --format '.items[0].articles[:5]'\napi 'https://example.org/api/resource' -X POST --input request.json -H 'Content-Type: application/json'",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if out.Type != string(output.JSONType) && out.Type != string(output.JQType) {
				return fmt.Errorf("--output must be json or jq")
			}
			if out.Type == string(output.JQType) && out.Format == "" {
				return fmt.Errorf("--output jq requires a --format expression")
			}
			if raw && (out.Format != "" || out.Type == string(output.JQType)) {
				return fmt.Errorf("--raw cannot be combined with JSON formatting")
			}
			if input != "" && len(fields) > 0 {
				return fmt.Errorf("use either --field or --input")
			}
			requestHeaders := make(http.Header)
			for _, line := range headers {
				name, value, ok := strings.Cut(line, ":")
				name = strings.TrimSpace(name)
				if !ok || name == "" {
					return fmt.Errorf("headers must use 'Name: value'")
				}
				requestHeaders.Add(name, strings.TrimSpace(value))
			}
			values := make(map[string]string, len(fields))
			for _, field := range fields {
				name, value, ok := strings.Cut(field, "=")
				if !ok || name == "" {
					return fmt.Errorf("fields must use name=value")
				}
				if _, ok := values[name]; ok {
					return fmt.Errorf("duplicate field %q", name)
				}
				values[name] = value
			}
			verb := strings.ToUpper(method)
			if verb == "" {
				verb = http.MethodGet
				if input != "" || len(fields) > 0 {
					verb = http.MethodPost
				}
			}
			var body io.Reader
			endpoint := args[0]
			if len(fields) > 0 {
				if verb == http.MethodGet || verb == http.MethodHead {
					// Add query fields without changing escaped endpoint paths.
					separator := "?"
					if strings.Contains(endpoint, "?") {
						separator = "&"
					}
					query := make([]string, 0, len(fields))
					for _, field := range fields {
						name, value, _ := strings.Cut(field, "=")
						query = append(query, url.QueryEscape(name)+"="+url.QueryEscape(value))
					}
					endpoint += separator + strings.Join(query, "&")
				} else {
					encoded, err := json.Marshal(values)
					if err != nil {
						return err
					}
					body = bytes.NewReader(encoded)
					if requestHeaders.Get("Content-Type") == "" {
						requestHeaders.Set("Content-Type", "application/json")
					}
				}
			}
			if input != "" {
				if input == "-" {
					body = cmd.InOrStdin()
				} else {
					file, err := os.Open(input)
					if err != nil {
						return err
					}
					defer file.Close()
					body = file
				}
				if requestHeaders.Get("Content-Type") == "" {
					requestHeaders.Set("Content-Type", "application/json")
				}
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()
			resp, err := restapi.Request(ctx, verb, baseURL, endpoint, requestHeaders, body)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if include {
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", resp.Proto, resp.Status)
				if err := resp.Header.Write(cmd.OutOrStdout()); err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			if err := restapi.CheckStatus(resp); err != nil {
				return err
			}
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			if len(bytes.TrimSpace(data)) == 0 {
				return nil
			}
			if raw {
				_, err = cmd.OutOrStdout().Write(data)
				return err
			}
			if !json.Valid(data) {
				if out.Type == string(output.JQType) || out.Format != "" {
					return fmt.Errorf("response is not JSON; cannot apply --format")
				}
				_, err = cmd.OutOrStdout().Write(data)
				return err
			}
			var result interface{}
			if err := json.Unmarshal(data, &result); err != nil {
				return err
			}
			out.Print(cmd, result)
			return nil
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&baseURL, "base-url", restapi.BaseURL, "Base URL for relative endpoints")
	flags.StringVarP(&method, "method", "X", "", "HTTP method (GET, or POST when --field/--input is provided)")
	flags.StringArrayVarP(&headers, "header", "H", nil, "Request header as 'Name: value' (repeatable)")
	flags.StringArrayVarP(&fields, "field", "f", nil, "String field as name=value (repeatable)")
	flags.StringVar(&input, "input", "", "Request body file, or - for stdin")
	flags.BoolVar(&raw, "raw", false, "Print the response body unchanged")
	flags.BoolVarP(&include, "include", "i", false, "Include the response status and headers")
	flags.DurationVar(&timeout, "timeout", 20*time.Second, "HTTP request timeout")
	out.AddFlagsWithOpts(cmd, output.WithDefaultOutput(output.JSONType), output.WithFilterFlagDisabled())
	return cmd
}
