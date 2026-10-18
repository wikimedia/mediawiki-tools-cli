package phabricator

import (
	"github.com/spf13/cobra"
	mwconfig "gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func newPhabricatorAuthLogoutCmd(site *string) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove saved Phabricator credentials and disable legacy login for this site",
		RunE: func(cmd *cobra.Command, args []string) error {
			siteName := resolveAuthSite(*site)
			if *site == "" && mwconfig.State().EffectiveKoanf.String("phabricator.default_site") == "" {
				if cfg, err := loadConfig(""); err == nil {
					siteName = cfg.SiteName
				}
			}
			prefix := "phabricator.sites." + siteName + "."
			keys := []string{prefix + "key"}
			if *site == "" {
				keys = append(keys, "phabricator.key")
			}
			for _, key := range keys {
				credential, err := mwconfig.CredentialForKey(key)
				if err != nil {
					return err
				}
				if err := credential.Delete(); err != nil {
					return err
				}
			}
			disk := mwconfig.State().OnDiskKoanf.Copy()
			disk.Delete(prefix + "username")
			if err := disk.Set(prefix+"logged_out", true); err != nil {
				return err
			}
			if *site == "" {
				disk.Delete("phabricator.username")
				if err := disk.Set("phabricator.default_site", siteName); err != nil {
					return err
				}
			}
			if err := mwconfig.PutDiskConfig(disk); err != nil {
				return err
			}
			cmd.Printf("Logged out from site %q; legacy phab.cfg credentials for this site are disabled until login\n", siteName)
			return nil
		},
	}
}
