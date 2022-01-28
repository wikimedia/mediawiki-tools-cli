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

func debugAttachToCmd(rootCmd *cobra.Command) {
	debugCmd := NewDebugCmd()
	rootCmd.AddCommand(debugCmd)
	debugEventsCmd := NewDebugEventsCmd()
	debugCmd.AddCommand(NewDebugEventsCmd())
	debugEventsCmd.AddCommand(NewDebugEventsEmitCmd())
}
