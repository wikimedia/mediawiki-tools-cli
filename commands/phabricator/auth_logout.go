package phabricator

import (
	"github.com/spf13/cobra"
	mwconfig "gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func newPhabricatorAuthLogoutCmd(site *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Logout of Wikimedia Phabricator",
		RunE: func(cmd *cobra.Command, args []string) error {
			siteName := resolveAuthSite(*site)

			for _, key := range []string{"username", "key"} {
				if err := mwconfig.DeleteKeyValueFromDisk(phabSiteConfigKey(siteName, key)); err != nil {
					return err
				}
			}

			if *site == "" {
				for _, key := range []string{"username", "key"} {
					if err := mwconfig.DeleteKeyValueFromDisk("phabricator." + key); err != nil {
						return err
					}
				}
			}

			cmd.Printf("Logged out from site %q\n", siteName)
			return nil
		},
	}

	return cmd
}
