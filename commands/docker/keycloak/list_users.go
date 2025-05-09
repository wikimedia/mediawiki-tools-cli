package keycloak

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dockercompose"
)

func NewKeycloakListUsersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "users <realmname>",
		Short: "List keycloak users in a realm",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().EnsureReady()
			keycloakLogin()
			err := mwdd.DefaultForUser().DockerCompose().Exec("keycloak", dockercompose.ExecOptions{
				User: "root",
				CommandAndArgs: []string{
					"/opt/keycloak/bin/kcadm.sh",
					"get",
					"users",
					"--target-realm", args[0],
					"--fields", "username",
					"--format", "csv",
					"--noquotes",
				},
			})
			if err != nil {
				panic(err)
			}
		},
	}
	return cmd
}
