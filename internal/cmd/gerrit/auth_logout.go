package gerrit

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewGerritAuthLogoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Logout of Wikimedia Gerrit from HTTP credentials",
		Run: func(cmd *cobra.Command, args []string) {
			config := &Config{}
			config.Write()
			logrus.Info("Logged out")
		},
	}
	return cmd
}
