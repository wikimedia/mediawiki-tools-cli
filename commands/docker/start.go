package docker

import (
	_ "embed"

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
			m := mwdd.DefaultForUser()
			return resumeDevelopmentServices(m, m.DockerCompose())
		},
	}
	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Control"
	return cmd
}
