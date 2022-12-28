package debug

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/eventlogging"
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

func NewDebugEventsEmitCmd() *cobra.Command {
	return &cobra.Command{
		Hidden:  debugCommandsAreHidden(),
		Use:     "submit",
		Aliases: []string{"emit"},
		Short:   "Submit events now",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Submitting events")
			eventlogging.EmitEvents()
		},
	}
}

func NewDebugEventsCatCmd() *cobra.Command {
	return &cobra.Command{
		Hidden:  debugCommandsAreHidden(),
		Use:     "cat",
		Aliases: []string{"list"},
		Short:   "List events pending submission",
		Run: func(cmd *cobra.Command, args []string) {
			for _, line := range eventlogging.RawEvents() {
				fmt.Println(line)
			}
		},
	}
}
