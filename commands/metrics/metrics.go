package metrics

import "github.com/spf13/cobra"

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "metrics", GroupID: "service", Short: "Query Wikimedia public analytics data"}
	cmd.AddCommand(NewPageviewsCmd())
	return cmd
}
