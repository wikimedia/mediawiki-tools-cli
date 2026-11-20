package dev

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/browser"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
)

func newDashboardCmd() *cobra.Command {
	var open bool
	cmd := &cobra.Command{
		Use:     "dashboard",
		GroupID: "core",
		Short:   "Show the dashboard URL for your development environment",
		Long:    "Show the published dashboard URL for the selected development context, or the configured URL when the gateway is offline. Open it to see running services and installed wikis.",
		Example: cobrautil.NormalizeExample("dashboard\ndashboard --open\ndashboard --context work"),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m := devenv.DefaultForUser()
			status, statusErr := m.GatewayStatus(cmd.Context())
			url := status.URL
			if url == "" {
				var err error
				url, err = m.DashboardURL()
				if err != nil {
					return fmt.Errorf("%w. %s", err, dashboardNextStep(cmd, status.State))
				}
			}
			fmt.Fprintln(cmd.OutOrStdout(), url)
			if open {
				if statusErr == nil && (status.State == devenv.GatewayMissing || status.State == devenv.GatewayStopped || status.State == devenv.GatewayDashboardStopped) {
					return fmt.Errorf("dashboard is unavailable (%s). %s", status.State, dashboardNextStep(cmd, status.State))
				}
				if !dashboardResponds(cmd.Context(), url) {
					if statusErr != nil {
						return fmt.Errorf("dashboard did not respond and Docker is unavailable: %w", statusErr)
					}
					return fmt.Errorf("dashboard did not respond. %s", dashboardNextStep(cmd, devenv.GatewayStarting))
				}
				return browser.OpenURL(url)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&open, "open", false, "Open the dashboard in your browser")
	return cmd
}
