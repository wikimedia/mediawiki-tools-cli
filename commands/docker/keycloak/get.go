package keycloak

import (
	_ "embed"

	"github.com/spf13/cobra"
)

func NewKeycloakGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get metadata for keycloak realm, client, or user",
	}
	cmd.AddCommand(NewKeycloakGetRealmCmd())
	cmd.AddCommand(NewKeycloakGetClientCmd())
	cmd.AddCommand(NewKeycloakGetClientSecretCmd())
	cmd.AddCommand(NewKeycloakGetUserCmd())
	return cmd
}
