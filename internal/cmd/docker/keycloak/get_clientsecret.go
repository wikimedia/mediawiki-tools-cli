package keycloak

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
)

func NewKeycloakGetClientSecretCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clientsecret <clientname> <realmname>",
		Short: "Get client secret for keycloak client in a realm",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().EnsureReady()
			keycloakLogin()
			mwdd.DefaultForUser().Exec("keycloak", []string{
				"/mwdd/get_client_secret.sh",
				args[0],
				args[1],
			}, "root")
		},
	}
	return cmd
}
