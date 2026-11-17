package phabricator

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

var conduitMethodPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*(\.[A-Za-z][A-Za-z0-9_]*)+$`)

func newPhabricatorAPICmd(site *string) *cobra.Command {
	var fields []string
	var input string
	out := &output.Output{}
	cmd := &cobra.Command{
		Annotations: commandmeta.Annotations(commandmeta.Hints{
			ToolAnnotations: commandmeta.ToolAnnotations{DestructiveHint: true, OpenWorldHint: true},
			DataAccess:      []commandmeta.DataAccess{commandmeta.DataUnknown},
		}),
		Use:   "api <method>",
		Short: "Call a Phabricator Conduit API method",
		Long: `Call a Conduit API method using a saved Phabricator profile.
Authentication and the HTTP POST request are handled automatically.
Prints the result as JSON, without the Conduit response envelope.
HTTP and Conduit errors return a failing exit status.

Use repeatable --field flags for string parameters, or --input FILE (or - for
stdin) for a JSON object containing typed or nested parameters.
This is a low-level interface: methods that modify data run without confirmation.`,
		Example: "api conduit.ping\n# \"phab1005\" (responding server; may vary)\napi conduit.getcapabilities\napi user.query -f limit=1\napi project.search --input params.json\napi project.search --input -\napi conduit.getcapabilities --output jq --format 'keys'",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !conduitMethodPattern.MatchString(args[0]) {
				return fmt.Errorf("invalid Conduit method %q (expected application.method, not a URL)", args[0])
			}
			if out.Type != string(output.JSONType) && out.Type != string(output.JQType) {
				return fmt.Errorf("--output must be json or jq")
			}
			if out.Type == string(output.JQType) && out.Format == "" {
				return fmt.Errorf("--output jq requires --format")
			}
			if err := output.ValidateJSONFormat(out.Format); err != nil {
				return err
			}
			params := make(map[string]interface{})
			if cmd.Flags().Changed("input") {
				reader := cmd.InOrStdin()
				if input != "-" {
					file, err := os.Open(input)
					if err != nil {
						return fmt.Errorf("read Conduit input: %w", err)
					}
					defer file.Close()
					reader = file
				}
				data, err := io.ReadAll(reader)
				if err != nil {
					return fmt.Errorf("read Conduit input: %w", err)
				}
				var object map[string]json.RawMessage
				if err := json.Unmarshal(data, &object); err != nil {
					return fmt.Errorf("parse Conduit input JSON object: %w", err)
				}
				if object == nil {
					return fmt.Errorf("Conduit input must be a JSON object, not null")
				}
				for key, value := range object {
					params[key] = value
				}
			}
			for _, field := range fields {
				name, value, ok := strings.Cut(field, "=")
				if !ok || name == "" {
					return fmt.Errorf("fields must use name=value")
				}
				if _, exists := params[name]; exists {
					return fmt.Errorf("duplicate field %q", name)
				}
				params[name] = value
			}
			if _, exists := params["__conduit__"]; exists {
				return fmt.Errorf("__conduit__ is reserved; authentication comes from the selected profile")
			}
			client, _, err := clientFromCommand(cmd, *site)
			if err != nil {
				return err
			}
			result, err := client.post(args[0], params)
			if err != nil {
				return err
			}
			return output.PrintJSONChecked(result, out.Format, out.Type == string(output.JQType), cmd.OutOrStdout())
		},
	}
	cmd.Flags().StringArrayVarP(&fields, "field", "f", nil, "String parameter as name=value (repeatable)")
	cmd.Flags().StringVar(&input, "input", "", "Read a JSON parameter object from a file, or - for stdin")
	cmd.MarkFlagsMutuallyExclusive("field", "input")
	out.AddFlagsWithOpts(cmd, output.WithDefaultOutput(output.JSONType), output.WithFilterFlagDisabled())
	return cmd
}
