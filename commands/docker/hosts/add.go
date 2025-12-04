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
		Short: "Adds development environment hosts into your system hosts fils",
		Run: func(cmd *cobra.Command, args []string) {
			changeResult := hosts.AddHosts(
				IP,
				append(
					[]string{
						// TODO generate these by reading the yml files?
						"proxy.local.wmftest.net",
						"eventlogging.local.wmftest.net",
						"adminer.local.wmftest.net",
						"mailhog.local.wmftest.net",
						"graphite.local.wmftest.net",
						"keycloak.local.wmftest.net",
						"phpmyadmin.local.wmftest.net",
						"default.mediawiki.local.wmftest.net",
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
