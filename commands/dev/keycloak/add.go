package keycloak

import (
	_ "embed"

	"github.com/spf13/cobra"
)

func NewKeycloakAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a keycloak realm, client, or user",
	}
	cmd.AddCommand(NewKeycloakAddRealmCmd())
	cmd.AddCommand(NewKeycloakAddClientCmd())
	cmd.AddCommand(NewKeycloakAddUserCmd())
	return cmd
}
