package enterprise

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

func newAPICmd(profile *string) *cobra.Command {
	var method, input string
	out := output.Output{}
	cmd := &cobra.Command{
		Use: "api <path>", Short: "Make an authenticated Enterprise data API request",
		Long:    "Call a Wikimedia Enterprise data endpoint using the selected account and automatic token refresh. Paths are relative to /v2, or may start with /v2/. GET is the default; --input selects POST unless --method is explicit. Input is a JSON file or '-' for stdin. HEAD returns public object headers. Use the download and stream commands for binary transfers and long-lived realtime connections.",
		Example: "api projects\napi articles/Squirrel --method POST --input query.json\napi /v2/snapshots/enwiki_namespace_0/download --method HEAD", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := apiPath(args[0])
			if err != nil {
				return err
			}
			method = strings.ToUpper(method)
			if method != http.MethodGet && method != http.MethodPost && method != http.MethodHead {
				return fmt.Errorf("--method must be GET, POST or HEAD")
			}
			var body any
			if input != "" {
				if method == http.MethodHead {
					return fmt.Errorf("HEAD does not accept --input")
				}
				reader := cmd.InOrStdin()
				if input != "-" {
					file, err := os.Open(input)
					if err != nil {
						return err
					}
					defer file.Close()
					reader = file
				}
				data, err := io.ReadAll(io.LimitReader(reader, (1<<20)+1))
				if err != nil {
					return fmt.Errorf("could not read API input")
				}
				if len(data) > 1<<20 {
					return fmt.Errorf("API input exceeds 1 MiB")
				}
				decoder := json.NewDecoder(bytes.NewReader(data))
				decoder.UseNumber()
				if err := decoder.Decode(&body); err != nil {
					return fmt.Errorf("API input must be a JSON object")
				}
				if _, ok := body.(map[string]any); !ok || decoder.Decode(new(any)) != io.EOF {
					return fmt.Errorf("API input must be one JSON object")
				}
				if !cmd.Flags().Changed("method") {
					method = http.MethodPost
				}
			}
			if out.Type != "json" && out.Type != "jq" {
				return fmt.Errorf("--output must be json or jq")
			}
			if err := output.ValidateJSONFormat(out.Format); err != nil {
				return err
			}
			c, err := client(*profile)
			if err != nil {
				return err
			}
			value, err := c.JSONMethod(cmd.Context(), method, p, body)
			if err != nil {
				return err
			}
			return output.PrintJSONChecked(value, out.Format, out.Type == "jq", cmd.OutOrStdout())
		},
	}
	cmd.Flags().StringVarP(&method, "method", "X", http.MethodGet, "HTTP method: GET, POST or HEAD")
	cmd.Flags().StringVar(&input, "input", "", "Read a JSON request body from a file or '-' for stdin")
	out.AddFlagsWithOpts(cmd, output.WithFilterFlagDisabled())
	return cmd
}

func apiPath(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil || raw == "" || parsed.IsAbs() || parsed.Host != "" || strings.Contains(raw, "#") {
		return "", fmt.Errorf("provide an Enterprise /v2/ API path, without a hostname")
	}
	if !strings.HasPrefix(raw, "/") {
		raw = "/v2/" + raw
	}
	u, err := url.ParseRequestURI(raw)
	if err != nil || u.IsAbs() || u.Host != "" || u.Fragment != "" || !strings.HasPrefix(u.Path, "/v2/") {
		return "", fmt.Errorf("provide an Enterprise /v2/ API path, without a hostname")
	}
	for _, part := range strings.Split(u.Path, "/") {
		if part == "." || part == ".." {
			return "", fmt.Errorf("API path cannot contain traversal components")
		}
	}
	return u.String(), nil
}
