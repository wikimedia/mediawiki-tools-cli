package dev

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
)

func NewDevStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "stop",
		GroupID: "core",
		Aliases: []string{"suspend"},
		Short:   "Stop all currently running containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := devenv.DefaultForUser()
			return m.WithServiceLifecycle(cmd.Context(), func() error { return suspendDevelopmentServices(m, m.DockerCompose()) })
		},
	}
	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Control"
	return cmd
}
