package dev

import (
	_ "embed"
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
)

func NewDevStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "start",
		GroupID: "core",
		Aliases: []string{"resume"},
		Short:   "Start containers that were running before",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := devenv.DefaultForUser()
			if err := resumeDevelopmentServices(m, m.DockerCompose()); err != nil {
				return err
			}
			if url := m.RunningDashboardURL(); url != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Dashboard: %s\n", url)
			}
			return nil
		},
	}
	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Control"
	return cmd
}
