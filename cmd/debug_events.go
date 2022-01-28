package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/eventlogging"
)

func NewDebugEventsCmd() *cobra.Command {
	return &cobra.Command{
		Hidden: true,
		Use:    "events",
	}
}

func NewDebugEventsEmitCmd() *cobra.Command {
	return &cobra.Command{
		Hidden: true,
		Use:    "emit",
		Short:  "Emit events now",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Emitting events")
			eventlogging.EmitEvents()
		},
	}
}
