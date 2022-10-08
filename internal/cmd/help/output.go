package help

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

//go:embed output.md
var output string

func NewOutputTopicCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "output",
		Short: "How to use --output, --filter and --format",
		Long:  cli.RenderMarkdown(output),
	}
	return cmd
}
