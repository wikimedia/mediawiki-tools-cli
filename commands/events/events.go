package events

import "github.com/spf13/cobra"

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "events", GroupID: "service", Short: "Follow public Wikimedia event streams"}
	cmd.AddCommand(NewTailCmd())
	return cmd
}
