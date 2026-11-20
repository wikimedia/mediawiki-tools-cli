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
		Long:    "Show the configured local dashboard URL for the selected development context. Open it to see running services and installed wikis.",
		Example: cobrautil.NormalizeExample("dashboard\ndashboard --open\ndashboard --context work"),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			url, err := devenv.DefaultForUser().DashboardURL()
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), url)
			if open {
				return browser.OpenURL(url)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&open, "open", false, "Open the dashboard in your browser")
	return cmd
}
