package keycloak

import (
	_ "embed"

	"github.com/spf13/cobra"
)

func NewKeycloakListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List keycloak realms, clients, or users",
	}
	cmd.AddCommand(NewKeycloakListRealmsCmd())
	cmd.AddCommand(NewKeycloakListClientsCmd())
	cmd.AddCommand(NewKeycloakListUsersCmd())
	return cmd
}
