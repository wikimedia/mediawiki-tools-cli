package phabricator

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	mwconfig "gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

const (
	defaultPhabricatorURL  = "https://phabricator.wikimedia.org"
	defaultPhabricatorSite = "wikimedia"
)

func addAuthCmd(parent *cobra.Command, site *string) {
	parent.AddCommand(newPhabricatorAuthCmd(site))
}

func newPhabricatorAuthCmd(site *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate mw with Wikimedia Phabricator",
	}
	cmd.AddCommand(newPhabricatorAuthLoginCmd(site))
	cmd.AddCommand(newPhabricatorAuthLogoutCmd(site))
	cmd.AddCommand(newPhabricatorAuthStatusCmd(site))
	return cmd
}

func resolveAuthSite(site string) string {
	if value := strings.TrimSpace(site); value != "" {
		return value
	}
	cfg := mwconfig.State()
	if value := strings.TrimSpace(cfg.EffectiveKoanf.String("phabricator.default_site")); value != "" {
		return value
	}
	if value := strings.TrimSpace(cfg.OnDiskKoanf.String("phabricator.default_site")); value != "" {
		return value
	}
	return defaultPhabricatorSite
}

func phabSiteConfigKey(site, key string) string {
	return fmt.Sprintf("phabricator.sites.%s.%s", site, key)
}

func validatePhabricatorCredentials(cmd *cobra.Command, cfg *PhabConfig) (string, error) {
	client := newConduitClient(cfg)
	result, err := client.post("user.whoami", map[string]interface{}{})
	if err != nil {
		return "", err
	}

	var whoami struct {
		UserName string `json:"userName"`
	}
	if err := json.Unmarshal(result, &whoami); err != nil {
		return "", err
	}
	if strings.TrimSpace(whoami.UserName) == "" {
		return "", fmt.Errorf("received empty username from user.whoami")
	}

	return whoami.UserName, nil
}

func tokenMask(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	if len(token) <= 8 {
		return "********"
	}
	return token[:8] + "********"
}
