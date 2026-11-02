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
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("ip") {
				IP = hosts.LocalIP()
			}
			return handleChangeResult(cmd, hosts.RemoveHostsWithSuffix(IP, "local.wmftest.net", true))
		},
	}
	cmd.Flags().StringVar(&IP, "ip", "", "IP address to use (automatically detected when omitted)")
	return cmd
}
