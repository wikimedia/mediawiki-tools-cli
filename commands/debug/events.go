package debug

import (
	"github.com/spf13/cobra"
)

func DebugEventsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Hidden:  debugCommandsAreHidden(),
		Short:   "Debug events / telemetry",
		Use:     "events",
		Aliases: []string{"telemetry"},
	}
	cmd.AddCommand(DebugEventsEmitCmd())
	cmd.AddCommand(DebugEventsCatCmd())
	return cmd
}
