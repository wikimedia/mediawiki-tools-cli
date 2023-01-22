package keycloak

import (
	_ "embed"

	"github.com/spf13/cobra"
	mwdd "gitlab.wikimedia.org/repos/releng/cli/internal/mwdd"
)

func NewKeycloakAddUserCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user <username> <password> <realmname>",
		Short: "Add a keycloak user to a realm with a temporary password",
		Args:  cobra.MinimumNArgs(3),
		Run: func(cmd *cobra.Command, args []string) {
			mwdd.DefaultForUser().EnsureReady()
			keycloakLogin()
			mwdd.DefaultForUser().Exec("keycloak", []string{
				"/mwdd/create_user.sh",
				args[0],
				args[1],
				args[2],
			}, "root")
		},
	}
	return cmd
}
