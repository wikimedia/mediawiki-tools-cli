package wiki

import (
	"fmt"

	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

func validateResourceOutput(out *output.Output, titles bool) error {
	switch out.Type {
	case "json", "table", "auto":
		return nil
	case "titles":
		if titles && out.Format == "" {
			return nil
		}
	case "jq":
		if out.Format != "" {
			return nil
		}
		return fmt.Errorf("--output jq requires --format")
	}
	return fmt.Errorf("unsupported output format %q", out.Type)
}
