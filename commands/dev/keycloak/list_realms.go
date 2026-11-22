package keycloak

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dockercompose"
)

func NewKeycloakListRealmsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "realms",
		Short: "List keycloak realms",
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
					"get",
					"realms",
					"--fields", "realm",
					"--format", "csv",
					"--noquotes",
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
