package wikidata

import "github.com/spf13/cobra"

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "wikidata", GroupID: "service", Short: "Work with Wikidata services"}
	cmd.AddCommand(NewQueryCmd())
	return cmd
}
