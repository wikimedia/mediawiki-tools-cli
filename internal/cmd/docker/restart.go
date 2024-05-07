package docker

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
)

func NewMwddRestartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "restart",
		GroupID: "core",
		Short:   "Restart the running containers",
		Run: func(cmd *cobra.Command, args []string) {
			err := mwdd.DefaultForUser().DockerCompose().Restart([]string{})
			if err != nil {
				panic(err)
			}
		},
	}
	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Control"
	return cmd
}
