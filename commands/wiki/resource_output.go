package wiki

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

func validateResourceOutput(out *output.Output, titles bool) error {
	switch out.Type {
	case "json":
		return output.ValidateJSONFormat(out.Format)
	case "table", "auto":
		if out.Format != "" {
			return fmt.Errorf("--format requires --output json or jq")
		}
		return nil
	case "titles":
		if titles && out.Format == "" {
			return nil
		}
	case "jq":
		if out.Format != "" {
			return output.ValidateJSONFormat(out.Format)
		}
		return fmt.Errorf("--output jq requires --format")
	}
	return fmt.Errorf("unsupported output format %q", out.Type)
}

func printResourceOutput(cmd *cobra.Command, out *output.Output, objects interface{}) error {
	if out.Type == "json" || out.Type == "jq" {
		return output.PrintJSONChecked(objects, out.Format, out.Type == "jq", cmd.OutOrStdout())
	}
	return out.Print(cmd, objects)
}
