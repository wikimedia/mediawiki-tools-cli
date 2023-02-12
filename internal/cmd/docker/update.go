package docker

import (
	_ "embed"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
)

func NewMwddUpdateCmd() *cobra.Command {
	var forceRecreate bool
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update running containers",
		Run: func(cmd *cobra.Command, args []string) {
			runningServices := mwdd.DefaultForUser().ServicesWithStatus("running")
			stoppedServices := mwdd.DefaultForUser().ServicesWithStatus("stopped")
			existingServices := append(runningServices, stoppedServices...)
			if len(existingServices) == 0 {
				logrus.Info("No services to update")
				return
			}
			logrus.Infof("Updating %d services", len(existingServices))
			logrus.Tracef("Updating services: %v", existingServices)
			mwdd.DefaultForUser().Pull(existingServices)
			mwdd.DefaultForUser().UpDetached(runningServices, forceRecreate)
		},
	}
	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Control"
	cmd.Flags().BoolVar(&forceRecreate, "force-recreate", false, "Force recreation of containers")
	return cmd
}
