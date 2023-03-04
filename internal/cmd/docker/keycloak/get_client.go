package keycloak

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
)

func NewKeycloakGetClientCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "client <clientname> <realmname>",
		Short: "Get metadata for keycloak client in a realm",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().EnsureReady()
			keycloakLogin()
			mwdd.DefaultForUser().Exec("keycloak", []string{
				"/opt/keycloak/bin/kcadm.sh",
				"get",
				"clients",
				"--query", "clientId=" + args[0],
				"--target-realm", args[1],
			}, "root")
		},
	}
	return cmd
}
