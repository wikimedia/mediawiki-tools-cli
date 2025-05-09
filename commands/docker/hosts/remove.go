package hosts

import (
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/hosts"
)

func NewHostsRemoveCmd() *cobra.Command {
	IP := ""
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Removes development environment hosts from your system hosts file",
		Run: func(cmd *cobra.Command, args []string) {
			handleChangeResult(hosts.RemoveHostsWithSuffix(IP, "mwdd.localhost", true))
		},
	}
	cmd.Flags().StringVar(&IP, "ip", hosts.LocalIP(), "IP address to interact with hosts for")
	return cmd
}
