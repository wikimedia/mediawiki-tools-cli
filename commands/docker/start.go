package docker

import (
	_ "embed"

	"fmt"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
)

func NewMwddStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "start",
		GroupID: "core",
		Aliases: []string{"resume"},
		Short:   "Start containers that were running before",
		RunE: func(cmd *cobra.Command, args []string) error {
			services, servicesErr := mwdd.DefaultForUser().DockerCompose().ServicesWithStatus("stopped")
			if servicesErr != nil {
				return fmt.Errorf("list stopped development containers: %w", servicesErr)
			}
			err := mwdd.DefaultForUser().DockerCompose().Start(services)
			return lifecycleError("start", err)
		},
	}
	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Control"
	return cmd
}
