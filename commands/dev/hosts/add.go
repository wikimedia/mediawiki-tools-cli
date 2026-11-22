package hosts

import (
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/hosts"
)

func NewHostsAddCmd() *cobra.Command {
	IP := ""
	extraHosts := []string{}
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Adds development environment hosts into your system hosts fils",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("ip") {
				IP = hosts.LocalIP()
			}
			names, err := hostsToAdd(extraHosts)
			if err != nil {
				return err
			}
			changeResult := hosts.AddHosts(
				IP,
				names,
				true,
			)
			return handleChangeResult(cmd, changeResult)
		},
	}
	cmd.Flags().StringVar(&IP, "ip", "", "IP address to use (automatically detected when omitted)")
	cmd.Flags().StringSliceVar(&extraHosts, "host", []string{}, "Additional hostnames to add to the hosts file")
	return cmd
}

func hostsToAdd(extraHosts []string) ([]string, error) {
	usedHosts, err := devenv.DefaultForUser().UsedHosts()
	if err != nil {
		return nil, err
	}
	return append(
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
		append(usedHosts, extraHosts...)...,
	), nil
}
