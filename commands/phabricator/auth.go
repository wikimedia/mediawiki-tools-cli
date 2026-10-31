package phabricator

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/authprofile"
)

const (
	defaultPhabricatorURL  = "https://phabricator.wikimedia.org"
	defaultPhabricatorSite = "wikimedia"
)

var phabProfiles = authprofile.Manager{Service: "phabricator", Label: "Phabricator", SecretField: "key", Endpoint: defaultPhabricatorURL}

func newPhabricatorAuthCmd(site *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate mw with Wikimedia Phabricator",
	}
	cmd.AddCommand(newPhabricatorAuthLoginCmd(site))
	cmd.AddCommand(phabProfiles.RemoveCmd(site))
	cmd.AddCommand(newPhabricatorAuthStatusCmd(site))
	cmd.AddCommand(phabProfiles.ProfilesCmd())
	cmd.AddCommand(phabProfiles.DefaultCmd(site))
	return cmd
}

func resolveAuthSite(site string) string {
	if value := strings.TrimSpace(site); value != "" {
		return value
	}
	if value := phabProfiles.Default(); value != "" {
		return value
	}
	return defaultPhabricatorSite
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
	return "********"
}
