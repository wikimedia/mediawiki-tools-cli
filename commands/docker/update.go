package docker

import (
	_ "embed"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dockercompose"
)

func NewMwddUpdateCmd() *cobra.Command {
	var forceRecreate bool
	cmd := &cobra.Command{
		Use:     "update",
		GroupID: "core",
		Short:   "Update running containers",
		Run: func(cmd *cobra.Command, args []string) {
			runningServices, runningServicesErr := mwdd.DefaultForUser().DockerCompose().ServicesWithStatus("running")
			if runningServicesErr != nil {
				logrus.Panic(runningServicesErr)
			}
			stoppedServices, stoppedServicesErr := mwdd.DefaultForUser().DockerCompose().ServicesWithStatus("stopped")
			if stoppedServicesErr != nil {
				logrus.Panic(stoppedServicesErr)
			}
			existingServices := append(runningServices, stoppedServices...)
			if len(existingServices) == 0 {
				logrus.Info("No services to update")
				return
			}
			logrus.Infof("Updating %d services", len(existingServices))
			logrus.Tracef("Updating services: %v", existingServices)
			err := mwdd.DefaultForUser().DockerCompose().Pull(existingServices)
			if err != nil {
				panic(err)
			}
			err = mwdd.DefaultForUser().DockerCompose().Up(runningServices, dockercompose.UpOptions{
				Detached:      true,
				ForceRecreate: forceRecreate,
			})
			if err != nil {
				panic(err)
			}
		},
	}
	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Control"
	cmd.Flags().BoolVar(&forceRecreate, "force-recreate", false, "Force recreation of containers")
	return cmd
}
