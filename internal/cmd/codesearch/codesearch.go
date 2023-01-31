package codesearch

import (
	"github.com/spf13/cobra"
)

func NewCodeSearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "codesearch",
		Short:   "Search MediaWiki and Wikimedia code",
		Aliases: []string{"cs"},
		RunE:    nil,
	}
	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Service"
	cmd.AddCommand(NewCodeSearchSearchCmd())
	return cmd
}
