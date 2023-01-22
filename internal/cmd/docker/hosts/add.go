package hosts

import (
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/hosts"
)

func NewHostsAddCmd() *cobra.Command {
	IP := ""
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Adds development environment hosts into your system hosts file (might need sudo)",
		Run: func(cmd *cobra.Command, args []string) {
			changeResult := hosts.AddHosts(
				IP,
				append(
					[]string{
						// TODO generate these by reading the yml files?
						"proxy.mwdd.localhost",
						"eventlogging.mwdd.localhost",
						"adminer.mwdd.localhost",
						"mailhog.mwdd.localhost",
						"graphite.mwdd.localhost",
						"keycloak.mwdd.localhost",
						"phpmyadmin.mwdd.localhost",
						"default.mediawiki.mwdd.localhost",
					},
					mwdd.DefaultForUser().UsedHosts()...,
				),
				true,
			)
			handleChangeResult(changeResult)
		},
	}
	cmd.Flags().StringVar(&IP, "ip", hosts.LocalIP(), "IP address to interact with hosts for")
	return cmd
}
