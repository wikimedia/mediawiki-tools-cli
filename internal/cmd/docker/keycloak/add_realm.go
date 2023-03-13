package keycloak

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dockercompose"
)

func NewKeycloakAddRealmCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "realm <realmname>",
		Short: "Add a keycloak realm",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().EnsureReady()
			keycloakLogin()
			mwdd.DefaultForUser().DockerCompose().Exec("keycloak", dockercompose.ExecOptions{
				User: "root",
				CommandAndArgs: []string{
					"/opt/keycloak/bin/kcadm.sh",
					"create",
					"realms",
					"--set", "enabled=true",
					"--set", "realm=" + args[0],
				},
			})
		},
	}
	return cmd
}
