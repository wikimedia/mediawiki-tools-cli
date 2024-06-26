package docker

import (
	_ "embed"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
)

func NewMwddStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "start",
		GroupID: "core",
		Aliases: []string{"resume"},
		Short:   "Start containers that were running before",
		Run: func(cmd *cobra.Command, args []string) {
			services, servicesErr := mwdd.DefaultForUser().DockerCompose().ServicesWithStatus("stopped")
			if servicesErr != nil {
				logrus.Error(servicesErr)
			}
			err := mwdd.DefaultForUser().DockerCompose().Start(services)
			if err != nil {
				panic(err)
			}
		},
	}
	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Control"
	return cmd
}
