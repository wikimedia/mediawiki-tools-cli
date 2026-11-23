package hosts

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/hosts"
)

func NewHostsWhereCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "where",
		Short: "Tell you where your system hosts file is",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), hosts.FilePath())
			return nil
		},
	}
}
