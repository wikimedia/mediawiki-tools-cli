package keycloak

import (
	_ "embed"

	"github.com/spf13/cobra"
)

func NewKeycloakDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete keycloak realm, client, or user",
	}
	cmd.AddCommand(NewKeycloakDeleteRealmCmd())
	cmd.AddCommand(NewKeycloakDeleteClientCmd())
	cmd.AddCommand(NewKeycloakDeleteUserCmd())
	return cmd
}
