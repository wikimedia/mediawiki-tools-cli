package keycloak

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dockercompose"
)

func NewKeycloakGetUserCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user <username> <realmname>",
		Short: "Get metadata for keycloak user in a realm",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().EnsureReady()
			keycloakLogin()
			mwdd.DefaultForUser().DockerCompose().Exec("keycloak", dockercompose.ExecOptions{
				User: "root",
				CommandAndArgs: []string{
					"/opt/keycloak/bin/kcadm.sh",
					"get",
					"users",
					"--query", "username=" + args[0],
					"--target-realm", args[1],
				},
			})
		},
	}
	return cmd
}
