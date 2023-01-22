package keycloak

import (
	_ "embed"

	"github.com/spf13/cobra"
	mwdd "gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
)

func NewKeycloakAddClientCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "client <clientname> <realmname>",
		Short: "Add a keycloak client to a realm",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().EnsureReady()
			keycloakLogin()
			mwdd.DefaultForUser().Exec("keycloak", []string{
				"/mwdd/create_client.sh",
				args[0],
				args[1],
			}, "root")
		},
	}
	return cmd
}
