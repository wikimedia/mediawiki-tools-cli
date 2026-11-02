package hosts

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/hosts"
)

func NewHostsWritableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "writable",
		Short: "Checks if you can write to the needed hosts file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if hosts.Writable() {
				fmt.Fprintln(cmd.OutOrStdout(), "Hosts file writable by the current user")
			} else {
				return fmt.Errorf("hosts file not writable by the current user")
			}
			return nil
		},
	}
}
