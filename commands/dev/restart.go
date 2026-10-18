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
			return restartDevelopmentServices(devenv.DefaultForUser().DockerCompose())
		},
	}
	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Control"
	return cmd
}
