package keycloak

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
)

func NewKeycloakListRealmsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "realms",
		Short: "List keycloak realms",
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().EnsureReady()
			keycloakLogin()
			mwdd.DefaultForUser().Exec("keycloak", []string{
				"/opt/keycloak/bin/kcadm.sh",
				"get",
				"realms",
				"--fields", "realm",
				"--format", "csv",
				"--noquotes",
			}, "root")
		},
	}
	return cmd
}
