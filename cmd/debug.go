package cmd

import (
	"github.com/spf13/cobra"
)

func NewDebugCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "debug",
		Hidden: true,
	}
}

func debugAttachToCmd() *cobra.Command {
	debugCmd := NewDebugCmd()
	debugEventsCmd := NewDebugEventsCmd()
	debugCmd.AddCommand(NewDebugEventsCmd())
	debugEventsCmd.AddCommand(NewDebugEventsEmitCmd())
	return debugCmd
}
