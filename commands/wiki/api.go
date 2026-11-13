package wiki

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	mwclient "cgt.name/pkg/go-mwclient"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiapi"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiauth"
)

func wikiAPIClient(public bool) (*wikiapi.Client, error) {
	endpoint, err := wikiauth.Endpoint(wiki)
	if err != nil {
		return nil, err
	}
	client := wikiauth.HTTPClient()
	client.Transport = wikiAPIContextTransport{ctx: wikiContext}
	if _, err = wikiapi.New(endpoint, client); err != nil {
		return nil, err
	}
	if !public && !wikiAnon {
		w, e := mwclient.New(endpoint, cli.HTTPUserAgent())
		if e != nil {
			return nil, e
		}
		if err = loginIfCredentialsProvided(w, client); err != nil {
			return nil, err
		}
		if bearer, ok := client.Transport.(wikiauth.BearerTransport); ok {
			bearer.IncludeREST = true
			client.Transport = bearer
		}
	}
	api, err := wikiapi.New(endpoint, client)
	if err == nil {
		api.AssertUser = !public && !wikiAnon && hasWikiCredentials()
	}
	return api, err
}

// Password-login library requests do not accept a context; carry the command's
// deadline through the transport as well as through the final API request.
type wikiAPIContextTransport struct{ ctx context.Context }

func (t wikiAPIContextTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return http.DefaultTransport.RoundTrip(req.Clone(t.ctx))
}

type apiOutput struct {
	output.Output
	raw bool
}

func (o *apiOutput) flags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&o.raw, "raw", false, "Print response bytes unchanged")
	o.AddFlagsWithOpts(cmd, output.WithDefaultOutput(output.JSONType), output.WithFilterFlagDisabled())
}

func (o *apiOutput) validate() error {
	if o.Type != "json" && o.Type != "jq" {
		return fmt.Errorf("--output must be json or jq")
	}
	if o.Type == "jq" && o.Format == "" {
		return fmt.Errorf("--output jq requires --format")
	}
	if o.raw && (o.Type == "jq" || o.Format != "") {
		return fmt.Errorf("--raw cannot be combined with JSON formatting")
	}
	return output.ValidateJSONFormat(o.Format)
}

func (o *apiOutput) print(cmd *cobra.Command, data []byte) error {
	if err := o.validate(); err != nil {
		return err
	}
	if o.raw || !json.Valid(data) {
		if !o.raw && (o.Type == "jq" || o.Format != "") {
			return fmt.Errorf("response is not JSON")
		}
		_, err := cmd.OutOrStdout().Write(data)
		return err
	}
	if len(data) == 0 {
		return nil
	}
	var value interface{}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	return printResourceOutput(cmd, &o.Output, value)
}

func apiFields(fields []string, repeat bool) (url.Values, error) {
	values := make(url.Values)
	for _, field := range fields {
		key, value, ok := strings.Cut(field, "=")
		if !ok || key == "" {
			return nil, fmt.Errorf("fields must use name=value")
		}
		if _, exists := values[key]; exists && !repeat {
			return nil, fmt.Errorf("duplicate field %q; use the API's multi-value syntax", key)
		}
		values.Add(key, value)
	}
	return values, nil
}

func newWikiAPICmd() *cobra.Command {
	var fields []string
	var method, token string
	var timeout time.Duration
	out := apiOutput{}
	cmd := &cobra.Command{
		Use: "api ACTION", Aliases: []string{"action"}, Short: "Call the selected wiki's Action API",
		Long:    "Call an Action API module using the selected wiki and saved login. Requests default to GET, even with fields. Use --method POST for form-encoded writes. API errors return a failing exit status. Each call returns one response; pass continuation parameters explicitly for additional pages. Discovery is optional; refresh public parameter metadata with api refresh for offline completions.",
		Example: "  api query -f titles=Earth -f prop=info\n  api wbgetentities -f ids=Q42 --wiki www.wikidata.org\n  api edit --method POST --token csrf -f title=Sandbox -f text=Example -f summary=Test",
		Args:    cobra.ExactArgs(1), ValidArgsFunction: completeAPIActions,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := out.validate(); err != nil {
				return err
			}
			method = strings.ToUpper(method)
			if method != "GET" && method != "POST" {
				return fmt.Errorf("Action API method must be GET or POST")
			}
			values, err := apiFields(fields, false)
			if err != nil {
				return err
			}
			if values.Has("action") {
				return fmt.Errorf("supply the action positionally")
			}
			values.Set("action", args[0])
			if token != "" && (token != "csrf" || method != "POST") {
				return fmt.Errorf("--token csrf requires --method POST")
			}
			if token != "" && values.Has("token") {
				return fmt.Errorf("use either --token csrf or -f token=...")
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()
			wikiContext = ctx
			client, err := wikiAPIClient(false)
			if err != nil {
				return err
			}
			if token != "" {
				data, e := client.Action(ctx, http.MethodGet, url.Values{"action": {"query"}, "meta": {"tokens"}, "type": {"csrf"}})
				if e != nil {
					return e
				}
				var result struct {
					Query struct {
						Tokens struct {
							CSRFToken string `json:"csrftoken"`
						} `json:"tokens"`
					} `json:"query"`
				}
				if json.Unmarshal(data, &result) != nil || result.Query.Tokens.CSRFToken == "" {
					return fmt.Errorf("wiki returned no CSRF token")
				}
				values.Set("token", result.Query.Tokens.CSRFToken)
			}
			data, err := client.Action(ctx, method, values)
			if err != nil {
				return err
			}
			return out.print(cmd, data)
		},
	}
	cmd.Flags().StringArrayVarP(&fields, "field", "f", nil, "API parameter as name=value (repeatable)")
	cmd.Flags().StringVarP(&method, "method", "X", "GET", "HTTP method: GET or POST")
	cmd.Flags().StringVar(&token, "token", "", "Fetch and attach a token (csrf; POST only)")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "Total request timeout, including login")
	out.flags(cmd)
	_ = cmd.RegisterFlagCompletionFunc("field", completeAPIFields)
	cmd.AddCommand(newActionRefreshCmd(), newActionDescribeCmd())
	return cmd
}

func newWikiRESTCmd() *cobra.Command {
	var fields []string
	var input, contentType string
	var timeout time.Duration
	out := apiOutput{}
	cmd := &cobra.Command{
		Use: "rest METHOD PATH", Short: "Call the selected wiki's MediaWiki REST API",
		Long:    "Call an explicit HTTP method and path relative to the selected installation's rest.php. Leading slashes are REST-relative. Fields are query parameters; --input supplies a body without changing JSON types. Saved wiki profiles also apply to REST. External URLs and redirects are not followed. Discovery is optional.",
		Example: "  rest get /v1/page/Earth/bare\n  rest get /v1/search/page -f q=Earth\n  rest post /wikibase/v1/entities/items --input item.json",
		Args:    cobra.ExactArgs(2), ValidArgsFunction: completeRESTPaths,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := out.validate(); err != nil {
				return err
			}
			method := strings.ToUpper(args[0])
			if !validRESTMethod(method) {
				return fmt.Errorf("unsupported REST method %q", args[0])
			}
			fields, err := apiFields(fields, true)
			if err != nil {
				return err
			}
			var body io.Reader
			if input != "" {
				if method == "GET" || method == "HEAD" {
					return fmt.Errorf("--input requires a method with a request body")
				}
				if input == "-" {
					body = cmd.InOrStdin()
				} else {
					file, e := os.Open(input)
					if e != nil {
						return e
					}
					defer file.Close()
					body = file
				}
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()
			wikiContext = ctx
			client, err := wikiAPIClient(false)
			if err != nil {
				return err
			}
			headers := make(http.Header)
			if body != nil {
				headers.Set("Content-Type", contentType)
			}
			data, err := client.REST(ctx, method, args[1], fields, body, headers)
			if err != nil {
				return err
			}
			return out.print(cmd, data)
		},
	}
	cmd.Flags().StringArrayVarP(&fields, "field", "f", nil, "Query parameter as name=value (repeatable; repeated keys are preserved)")
	cmd.Flags().StringVar(&input, "input", "", "Request body file, or - for stdin")
	_ = cmd.MarkFlagFilename("input")
	cmd.Flags().StringVar(&contentType, "content-type", "application/json", "Content type for --input")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "Total request timeout, including login")
	out.flags(cmd)
	_ = cmd.RegisterFlagCompletionFunc("field", completeRESTFields)
	cmd.AddCommand(newRESTRefreshCmd(), newRESTDescribeCmd())
	return cmd
}

func validRESTMethod(method string) bool {
	switch method {
	case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS":
		return true
	}
	return false
}
