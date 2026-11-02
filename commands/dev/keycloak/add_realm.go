package keycloak

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dockercompose"
)

func NewKeycloakAddRealmCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "realm <realmname>",
		Short: "Add a keycloak realm",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := devenv.DefaultForUser().EnsureReady(); err != nil {
				return err
			}
			if err := keycloakLogin(); err != nil {
				return err
			}
			err := devenv.DefaultForUser().DockerCompose().Exec("keycloak", dockercompose.ExecOptions{
				User: "root",
				CommandAndArgs: []string{
					"/opt/keycloak/bin/kcadm.sh",
					"create",
					"realms",
					"--set", "enabled=true",
					"--set", "realm=" + args[0],
				},
			})
			if err != nil {
				return err
			}
			return nil
		},
	}
	return cmd
}
