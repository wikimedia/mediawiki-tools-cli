package debug

import (
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/agent"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/output"
)

func DebugAgentCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "agent",
		Short:   "Show automatic AI agent detection as JSON",
		Hidden:  true,
		Args:    cobra.NoArgs,
		Example: cobrautil.NormalizeExample("agent"),
		Annotations: map[string]string{
			cli.TelemetrySkipAnnotation: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			detected, err := agent.Detect()
			if err != nil {
				return err
			}
			result := struct {
				IsAgent bool    `json:"is_agent"`
				Agent   *string `json:"agent"`
			}{IsAgent: detected != nil}
			if detected != nil {
				result.Agent = &detected.Name
			}
			return output.PrintJSONChecked(result, ".", false, cmd.OutOrStdout())
		},
	}
}
