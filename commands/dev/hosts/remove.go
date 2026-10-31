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
			if !cmd.Flags().Changed("ip") {
				IP = hosts.LocalIP()
			}
			handleChangeResult(hosts.RemoveHostsWithSuffix(IP, "local.wmftest.net", true))
		},
	}
	cmd.Flags().StringVar(&IP, "ip", "", "IP address to use (automatically detected when omitted)")
	return cmd
}
