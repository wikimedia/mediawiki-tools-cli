package dev

import (
	_ "embed"

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
			return m.WithServiceLifecycle(cmd.Context(), func() error { return resumeDevelopmentServices(m, m.DockerCompose()) })
		},
	}
	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Control"
	return cmd
}
