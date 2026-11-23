package liftwing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/accesspolicy"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "liftwing", GroupID: "service", Short: "Discover and query public Wikimedia machine learning models",
		Long: "Discover documented Lift Wing prediction endpoints and send model-specific JSON requests without authentication. Anonymous rate limits apply. Models have different inputs, outputs and stability; inspect their specification and model card before use. Large language model chat endpoints are outside this command's prediction interface.",
	}
	models := &cobra.Command{Use: "models", Short: "Inspect documented prediction models"}
	models.AddCommand(newListCmd(), newInfoCmd())
	cmd.AddCommand(models, newPredictCmd())
	return cmd
}

func newListCmd() *cobra.Command {
	out := output.Output{}
	cmd := &cobra.Command{
		Use: "list", Short: "List prediction routes from the current public API specification", Args: cobra.NoArgs,
		Long:    "Fetch the public OpenAPI specification and list its documented prediction routes. Names containing {wiki} are route families; this does not imply support for every wiki. This is API documentation discovery, not a model health check. Use models info with a concrete name to see its inputs, examples and model card.",
		Example: "list\nlist --output json",
	}
	out.AddFlagsWithOpts(cmd, output.WithDefaultTTY(output.TableType), output.WithDefaultPipe(output.JSONType), output.WithFilterFlagDisabled(), output.WithTableBinding(&output.TableBinding{
		Headings: []string{"Model / family", "Endpoint"},
		ProcessObjects: func(objects any, table *output.Table) {
			for _, m := range objects.([]model) {
				table.AddRowS(m.Name, m.Endpoint)
			}
		},
	}))
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		if err := validateOutput(out, true); err != nil {
			return err
		}
		models, err := discover(cmd.Context())
		if err != nil {
			return err
		}
		if out.Type == "json" || out.Type == "jq" {
			return output.PrintJSONChecked(models, out.Format, out.Type == "jq", cmd.OutOrStdout())
		}
		out.Print(cmd, models)
		return nil
	}
	return publicCommand(cmd, false)
}

func newInfoCmd() *cobra.Command {
	out := output.Output{}
	cmd := &cobra.Command{Use: "info MODEL", Short: "Show a model's documented inputs, examples, responses and model card", Args: cobra.ExactArgs(1), Example: "info outlink-topic-model\ninfo enwiki-damaging"}
	out.AddFlagsWithOpts(cmd, output.WithDefaultOutput(output.JSONType), output.WithFilterFlagDisabled())
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := validateOutput(out, false); err != nil {
			return err
		}
		m, err := modelInfo(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		return output.PrintJSONChecked(m, out.Format, out.Type == "jq", cmd.OutOrStdout())
	}
	return publicCommand(cmd, false)
}

func newPredictCmd() *cobra.Command {
	var file string
	out := output.Output{}
	cmd := &cobra.Command{
		Use: "predict MODEL", Short: "Send a model-specific JSON object and print its prediction", Args: cobra.ExactArgs(1),
		Long:    "Send one JSON object to a documented public prediction route. --file - reads standard input; other values read a file. Input is limited to 4 MiB and responses to 16 MiB. Model-specific validation is performed by the service. Predictions preserve the API response and numeric identifiers. No automatic retries are made.",
		Example: "predict outlink-topic-model --file request.json\npredict revertrisk-multilingual --file - < revision.json",
	}
	cmd.Flags().StringVar(&file, "file", "-", "JSON request file, or - for standard input")
	out.AddFlagsWithOpts(cmd, output.WithDefaultOutput(output.JSONType), output.WithFilterFlagDisabled())
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := validateOutput(out, false); err != nil {
			return err
		}
		if !modelName.MatchString(args[0]) {
			return fmt.Errorf("invalid model name %q", args[0])
		}
		var reader io.Reader = cmd.InOrStdin()
		if file != "-" {
			f, err := os.Open(file)
			if err != nil {
				return err
			}
			defer f.Close()
			reader = f
		}
		data, err := readLimited(reader, maxInput)
		if err != nil {
			return err
		}
		var body map[string]json.RawMessage
		if err := json.Unmarshal(data, &body); err != nil || body == nil {
			return fmt.Errorf("request must be a single JSON object")
		}
		m, err := findModel(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		response, err := request(cmd.Context(), http.MethodPost, m.Endpoint, bytes.NewReader(data))
		if err != nil {
			return err
		}
		var result any
		decoder := json.NewDecoder(bytes.NewReader(response))
		decoder.UseNumber()
		if err := decoder.Decode(&result); err != nil {
			return fmt.Errorf("decode Lift Wing prediction: %w", err)
		}
		if decoder.Decode(new(any)) != io.EOF {
			return fmt.Errorf("Lift Wing returned multiple or invalid JSON values")
		}
		return output.PrintJSONChecked(result, out.Format, out.Type == "jq", cmd.OutOrStdout())
	}
	return publicCommand(cmd, true)
}

func validateOutput(out output.Output, table bool) error {
	if out.Type != "json" && out.Type != "jq" && !(table && (out.Type == "table" || out.Type == "auto")) {
		return fmt.Errorf("unsupported output format %q", out.Type)
	}
	return output.ValidateJSONFormat(out.Format)
}

func publicCommand(cmd *cobra.Command, supplied bool) *cobra.Command {
	accesspolicy.Support(cmd)
	access := []commandmeta.DataAccess{commandmeta.DataPublic}
	if supplied {
		access = append(access, commandmeta.DataSuppliedContent)
	}
	return commandmeta.Set(cmd, commandmeta.Hints{
		ToolAnnotations: commandmeta.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: !supplied, OpenWorldHint: true}, DataAccess: access,
	})
}
