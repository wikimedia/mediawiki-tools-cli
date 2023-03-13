package keycloak

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dockercompose"
)

func NewKeycloakDeleteUserCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user <username> <realmname>",
		Short: "Delete keycloak user in a realm",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().EnsureReady()
			keycloakLogin()
			mwdd.DefaultForUser().DockerCompose().Exec("keycloak", dockercompose.ExecOptions{
				User: "root",
				CommandAndArgs: []string{
					"/mwdd/delete_user.sh",
					args[0],
					args[1],
				},
			})
		},
	}
	return cmd
}
