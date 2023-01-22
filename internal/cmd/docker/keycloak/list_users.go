package keycloak

import (
	_ "embed"

	"github.com/spf13/cobra"
	mwdd "gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
)

func NewKeycloakListUsersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "users <realmname>",
		Short: "List keycloak users in a realm",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().EnsureReady()
			keycloakLogin()
			mwdd.DefaultForUser().Exec("keycloak", []string{
				"/opt/keycloak/bin/kcadm.sh",
				"get",
				"users",
				"--target-realm", args[0],
				"--fields", "username",
				"--format", "csv",
				"--noquotes",
			}, "root")
		},
	}
	return cmd
}
