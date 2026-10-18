package analytics

import "github.com/spf13/cobra"

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "analytics", GroupID: "service", Short: "Query the Wikimedia Analytics API"}
	cmd.AddCommand(NewPageviewsCmd())
	return cmd
}
