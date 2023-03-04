package keycloak

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
)

func NewKeycloakDeleteUserCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user <username> <realmname>",
		Short: "Delete keycloak user in a realm",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().EnsureReady()
			keycloakLogin()
			mwdd.DefaultForUser().Exec("keycloak", []string{
				"/mwdd/delete_user.sh",
				args[0],
				args[1],
			}, "root")
		},
	}
	return cmd
}
