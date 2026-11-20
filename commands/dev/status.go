package dev

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
)

func newDevStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		GroupID: "core",
		Short:   "Show gateway and dashboard status for the selected context",
		Long:    "Show the development context, gateway state, and dashboard URL, with a next step based on what is running.",
		Example: cobrautil.NormalizeExample("status\nstatus --context work"),
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			m := devenv.DefaultForUser()
			status, err := m.GatewayStatus(cmd.Context())
			state := string(status.State)
			next := dashboardNextStep(cmd, status.State)
			switch {
			case err != nil:
				state = "Docker unavailable"
				next = "Check that Docker is running with `docker info`."
			case status.State == devenv.GatewayRunning && !dashboardResponds(cmd.Context(), status.URL):
				state = "running, dashboard not responding yet"
				next = dashboardNextStep(cmd, devenv.GatewayStarting)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Context: %s\nGateway: %s\n", devenv.Context, state)
			if status.URL != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Dashboard: %s\n", status.URL)
			} else if url, err := m.DashboardURL(); err == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Dashboard (configured): %s\n", url)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "Dashboard: no URL configured")
			}
			fmt.Fprintln(cmd.OutOrStdout(), next)
		},
	}
}

func devContextCommand(cmd *cobra.Command, subcommand string) string {
	name := cmd.Root().Name() + " dev"
	if devenv.Context != "default" || cmd.Flags().Changed("context") {
		name += fmt.Sprintf(" --context %q", devenv.Context)
	}
	return name + " " + subcommand
}

func dashboardNextStep(cmd *cobra.Command, state devenv.GatewayState) string {
	switch state {
	case devenv.GatewayRunning:
		return "Open it with `" + devContextCommand(cmd, "dashboard --open") + "`."
	case devenv.GatewayStarting:
		return "Give the dashboard a moment, then retry `" + devContextCommand(cmd, "dashboard --open") + "`."
	case devenv.GatewayDashboardStopped:
		return "Start the dashboard with `" + devContextCommand(cmd, "traefik start") + "`."
	case devenv.GatewayStopped:
		services, saved, err := devenv.DefaultForUser().ResumeServices()
		if err == nil && saved && !slices.Contains(services, "traefik") {
			return "Start the gateway with `" + devContextCommand(cmd, "traefik start") + "`."
		}
		return "Resume the environment with `" + devContextCommand(cmd, "start") + "`."
	default:
		return "Set up an environment with the guide at `" + cmd.Root().Name() + " help guides dev setup`."
	}
}

func dashboardResponds(ctx context.Context, url string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	transport := &http.Transport{}
	defer transport.CloseIdleConnections()
	probe := &http.Client{
		Transport: transport,
		Timeout:   1500 * time.Millisecond,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := probe.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
