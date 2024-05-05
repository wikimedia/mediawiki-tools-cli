package docker

import (
	_ "embed"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dockercompose"
)

func NewMwddDestroyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "Destroy all containers and data",
		Run: func(cmd *cobra.Command, args []string) {
			err := mwdd.DefaultForUser().DockerCompose().Down(dockercompose.DownOptions{
				Volumes:       true,
				RemoveOrphans: true,
			})
			if err != nil {
				panic(err)
			}
			logrus.Debug("Removing used hosts file")
			mwdd.DefaultForUser().RemoveUsedHostsIfExists()
		},
	}
	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Control"
	return cmd
}
