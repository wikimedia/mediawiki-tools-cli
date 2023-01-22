package debug

import (
	"github.com/spf13/cobra"
)

func NewDebugEventsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Hidden:  debugCommandsAreHidden(),
		Short:   "Debug events / telemetry",
		Use:     "events",
		Aliases: []string{"telemetry"},
	}
	cmd.AddCommand(NewDebugEventsEmitCmd())
	cmd.AddCommand(NewDebugEventsCatCmd())
	return cmd
}
