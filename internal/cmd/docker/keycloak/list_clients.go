package keycloak

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
)

func NewKeycloakListClientsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clients <realmname>",
		Short: "List keycloak clients in a realm",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().EnsureReady()
			keycloakLogin()
			mwdd.DefaultForUser().Exec("keycloak", []string{
				"/opt/keycloak/bin/kcadm.sh",
				"get",
				"clients",
				"--target-realm", args[0],
				"--fields", "clientId",
				"--format", "csv",
				"--noquotes",
			}, "root")
		},
	}
	return cmd
}
