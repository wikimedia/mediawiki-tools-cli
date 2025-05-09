package codesearch

import (
	"github.com/spf13/cobra"
)

func NewCodeSearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "codesearch",
		GroupID: "service",
		Short:   "Search MediaWiki and Wikimedia code",
		Aliases: []string{"cs"},
		RunE:    nil,
	}
	cmd.AddCommand(NewCodeSearchSearchCmd())
	return cmd
}
