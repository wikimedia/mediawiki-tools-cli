package dev

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
)

func NewDevRestartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "restart",
		GroupID: "core",
		Short:   "Restart the running containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := devenv.DefaultForUser()
			return m.WithServiceLifecycle(cmd.Context(), func() error {
				inventory, err := m.DockerCompose().ServiceStatuses()
				if err != nil {
					return err
				}
				if err := m.EnsureServiceIntent(inventory); err != nil {
					return err
				}
				return restartDevelopmentServices(m.DockerCompose())
			})
		},
	}
	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Control"
	return cmd
}
