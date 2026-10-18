package gerrit

import (
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func NewGerritAuthLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove saved Gerrit HTTP credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			credential, err := config.CredentialForKey("gerrit.password")
			if err != nil {
				return err
			}
			if err := credential.Delete(); err != nil {
				return err
			}
			if err := config.DeleteKeyValueFromDisk("gerrit.username"); err != nil {
				return err
			}
			cmd.Println("Removed saved Gerrit credentials (environment overrides are unchanged)")
			return nil
		},
	}
}
