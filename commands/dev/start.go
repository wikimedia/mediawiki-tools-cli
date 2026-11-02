package dev

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
)

func NewDevStartCmd() *cobra.Command {
	var rememberRunning bool
	cmd := &cobra.Command{
		Use:     "start",
		GroupID: "core",
		Aliases: []string{"resume"},
		Short:   "Start containers that were running before",
		Long:    "Resume the services recorded when the environment was stopped. With --remember-running, initialize a missing resume record from currently running services, leaving an existing record unchanged.",
		Example: "start\nstart --remember-running",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := devenv.DefaultForUser()
			if rememberRunning {
				if err := rememberRunningDevelopmentServices(m, m.DockerCompose()); err != nil {
					return err
				}
			}
			return resumeDevelopmentServices(m, m.DockerCompose())
		},
	}
	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Control"
	cmd.Flags().BoolVar(&rememberRunning, "remember-running", false, "Initialize a missing resume record from currently running services")
	return cmd
}
