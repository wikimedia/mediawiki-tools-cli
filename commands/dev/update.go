package dev

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dockercompose"
)

func NewDevUpdateCmd() *cobra.Command {
	var forceRecreate bool
	cmd := &cobra.Command{
		Use:     "update",
		GroupID: "core",
		Short:   "Update running containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := devenv.DefaultForUser().PrepareGatewayWithOutput(cmd.ErrOrStderr()); err != nil {
				return err
			}
			runningServices, runningServicesErr := devenv.DefaultForUser().DockerCompose().ServicesWithStatus("running")
			if runningServicesErr != nil {
				return runningServicesErr
			}
			runningServices = devenv.WithoutTunnelServices(runningServices)
			stoppedServices, stoppedServicesErr := devenv.DefaultForUser().DockerCompose().ServicesWithStatus("stopped")
			if stoppedServicesErr != nil {
				return stoppedServicesErr
			}
			stoppedServices = devenv.WithoutTunnelServices(stoppedServices)
			existingServices := append(runningServices, stoppedServices...)
			if len(existingServices) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "No services to update")
				return nil
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Updating %d services\n", len(existingServices))
			slog.Log(context.Background(), slog.Level(-8), fmt.Sprintf("Updating services: %v", existingServices))
			compose := devenv.DefaultForUser().DockerCompose().WithIO(cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
			err := compose.Pull(existingServices)
			if err != nil {
				return err
			}
			// An empty Compose up selection means all configured services, including
			// optional services the user has never created.
			if len(runningServices) == 0 {
				return nil
			}
			err = compose.Up(runningServices, dockercompose.UpOptions{
				Detached:      true,
				ForceRecreate: forceRecreate,
			})
			return err
		},
	}
	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Control"
	cmd.Flags().BoolVar(&forceRecreate, "force-recreate", false, "Force recreation of containers")
	return cmd
}
