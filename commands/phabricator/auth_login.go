package phabricator

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	mwconfig "gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func newPhabricatorAuthLoginCmd(site *string) *cobra.Command {
	var username string
	var token string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login to Wikimedia Phabricator using API token",
		RunE: func(cmd *cobra.Command, args []string) error {
			siteName := resolveAuthSite(*site)

			if strings.TrimSpace(username) == "" {
				prompt := &survey.Input{Message: "Phabricator username:"}
				if err := survey.AskOne(prompt, &username); err != nil {
					return err
				}
			}
			username = strings.TrimSpace(username)
			if username == "" {
				return fmt.Errorf("username is required")
			}

			if strings.TrimSpace(token) == "" {
				cmd.Println("Create a token at https://phabricator.wikimedia.org/settings/user/<username>/page/apitokens/")
				prompt := &survey.Password{Message: "Phabricator API token:"}
				if err := survey.AskOne(prompt, &token); err != nil {
					return err
				}
			}
			token = strings.TrimSpace(token)
			if token == "" {
				return fmt.Errorf("API token is required")
			}

			cfg := &PhabConfig{
				SiteName: siteName,
				URL:      defaultPhabricatorURL,
				Username: username,
				Key:      token,
			}

			if existing, err := loadConfig(siteName); err == nil {
				if strings.TrimSpace(existing.URL) != "" {
					cfg.URL = existing.URL
				}
				if strings.TrimSpace(existing.DefaultProject) != "" {
					cfg.DefaultProject = existing.DefaultProject
				}
				if strings.TrimSpace(existing.CachePath) != "" {
					cfg.CachePath = existing.CachePath
				}
			}

			whoami, err := validatePhabricatorCredentials(cmd, cfg)
			if err != nil {
				var ignoreError bool
				prompt := &survey.Confirm{Message: "Could not validate credentials. Save them anyway?"}
				if askErr := survey.AskOne(prompt, &ignoreError); askErr != nil {
					return askErr
				}
				if !ignoreError {
					return fmt.Errorf("credentials not saved")
				}
			} else {
				cmd.Printf("Authenticated as %s\n", whoami)
			}

			if err := mwconfig.PutKeyValueOnDisk("phabricator.default_site", siteName); err != nil {
				return err
			}
			if err := mwconfig.PutKeyValueOnDisk(phabSiteConfigKey(siteName, "url"), cfg.URL); err != nil {
				return err
			}
			if err := mwconfig.PutKeyValueOnDisk(phabSiteConfigKey(siteName, "username"), cfg.Username); err != nil {
				return err
			}
			if err := mwconfig.PutKeyValueOnDisk(phabSiteConfigKey(siteName, "key"), cfg.Key); err != nil {
				return err
			}

			cmd.Printf("Saved credentials for site %q\n", siteName)
			cmd.Printf("Tip: if a legacy phab.cfg exists, it takes precedence over mwcli config.\n")
			return nil
		},
	}

	cmd.Flags().StringVarP(&username, "username", "u", "", "Phabricator username")
	cmd.Flags().StringVarP(&token, "token", "t", "", "Phabricator API token")

	return cmd
}
