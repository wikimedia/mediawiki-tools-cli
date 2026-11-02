package keycloak

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dockercompose"
)

func NewKeycloakAddUserCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user <username> <password> <realmname>",
		Short: "Add a keycloak user to a realm with a temporary password",
		Args:  cobra.MinimumNArgs(3),
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
					"/mwdd/create_user.sh",
					args[0],
					args[1],
					args[2],
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
