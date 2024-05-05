package docker

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
)

func NewMwddStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "stop",
		Aliases: []string{"suspend"},
		Short:   "Stop all currently running containers",
		Run: func(cmd *cobra.Command, args []string) {
			// Stop all containers that were running
			err := mwdd.DefaultForUser().DockerCompose().Stop([]string{})
			if err != nil {
				panic(err)
			}
		},
	}
	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Control"
	return cmd
}
