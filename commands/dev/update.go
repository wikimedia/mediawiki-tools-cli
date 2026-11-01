package dev

import (
	_ "embed"

	"github.com/sirupsen/logrus"
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
			m := devenv.DefaultForUser()
			return m.WithServiceLifecycle(cmd.Context(), func() error {
				if err := m.PrepareGateway(); err != nil {
					return err
				}
				return updateDevelopmentServices(m, m.DockerCompose(), forceRecreate)
			})
		},
	}
	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Control"
	cmd.Flags().BoolVar(&forceRecreate, "force-recreate", false, "Force recreation of containers")
	return cmd
}

type updateProject interface {
	ServiceStatuses() ([]dockercompose.ServiceStatus, error)
	Pull([]string) error
	Up([]string, dockercompose.UpOptions) error
}

func updateDevelopmentServices(m devenv.Environment, p updateProject, forceRecreate bool) error {
	inventory, err := p.ServiceStatuses()
	if err != nil {
		return err
	}
	if err := m.EnsureServiceIntent(inventory); err != nil {
		return err
	}
	var existing, running []string
	for _, service := range inventory {
		if service.OneShot || devenv.IsTunnelRuntimeService(service.Name) {
			continue
		}
		existing = append(existing, service.Name)
		if service.State == "running" {
			running = append(running, service.Name)
		}
	}
	if len(existing) == 0 {
		logrus.Info("No services to update")
		return nil
	}
	logrus.Infof("Updating %d existing services", len(existing))
	if err := p.Pull(existing); err != nil {
		return err
	}
	// An empty `up` target means all configured services. Pulling
	// images for a fully stopped environment must not start it.
	if len(running) == 0 {
		return nil
	}
	return p.Up(running, dockercompose.UpOptions{
		Detached: true, ForceRecreate: forceRecreate, NoDeps: true,
	})
}
