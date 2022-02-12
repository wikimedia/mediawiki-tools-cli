package debug

import (
	"github.com/spf13/cobra"
)

func NewDebugCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "debug",
		Hidden: true,
	}
	cmd.AddCommand(NewDebugEventsCmd())
	return cmd
}
