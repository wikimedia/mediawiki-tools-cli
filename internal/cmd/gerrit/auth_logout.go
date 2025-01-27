package gerrit

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func NewGerritAuthLogoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Logout of Wikimedia Gerrit from HTTP credentials",
		Run: func(cmd *cobra.Command, args []string) {
			c := config.State()
			if c.Effective.Gerrit.Username != "" {
				config.PutKeyValueOnDisk("gerrit.username", "")
			}
			if c.Effective.Gerrit.Password != "" {
				config.PutKeyValueOnDisk("gerrit.password", "")
			}
			logrus.Info("Logged out")
		},
	}
	return cmd
}
