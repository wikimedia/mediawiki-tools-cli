package dev

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
)

func NewDevStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "start",
		GroupID: "core",
		Aliases: []string{"resume"},
		Short:   "Start existing containers intended to run",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := devenv.DefaultForUser()
			if gateway, err := m.GatewayStatus(cmd.Context()); err == nil && !gateway.HasContainers {
				fmt.Fprintln(cmd.OutOrStdout(), "No services to resume.")
				fmt.Fprintln(cmd.OutOrStdout(), dashboardNextStep(cmd, devenv.GatewayMissing))
				return nil
			}
			var resumed bool
			err := m.WithServiceLifecycle(cmd.Context(), func() error {
				var err error
				resumed, err = resumeDevelopmentServices(m, m.DockerCompose())
				return err
			})
			if err != nil {
				return err
			}
			gateway, statusErr := m.GatewayStatus(cmd.Context())
			if !resumed {
				fmt.Fprintln(cmd.OutOrStdout(), "No services to resume.")
				if statusErr == nil && gateway.State == devenv.GatewayMissing {
					fmt.Fprintln(cmd.OutOrStdout(), dashboardNextStep(cmd, gateway.State))
				} else if gateway.State != devenv.GatewayRunning {
					fmt.Fprintf(cmd.OutOrStdout(), "Check the environment with `%s`.\n", devContextCommand(cmd, "status"))
				}
			}
			if gateway.State == devenv.GatewayRunning {
				fmt.Fprintf(cmd.OutOrStdout(), "Dashboard: %s\n", gateway.URL)
			}
			return nil
		},
	}
	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Control"
	return cmd
}
