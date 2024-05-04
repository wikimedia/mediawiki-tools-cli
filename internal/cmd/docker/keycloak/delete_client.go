package keycloak

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dockercompose"
)

func NewKeycloakDeleteClientCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "client <clientname> <realmname>",
		Short: "Delete keycloak client in a realm",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().EnsureReady()
			keycloakLogin()
			err := mwdd.DefaultForUser().DockerCompose().Exec("keycloak", dockercompose.ExecOptions{
				User: "root",
				CommandAndArgs: []string{
					"/mwdd/delete_client.sh",
					args[0],
					args[1],
				},
			})
			if err != nil {
				panic(err)
			}
		},
	}
	return cmd
}
