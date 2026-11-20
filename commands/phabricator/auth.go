package phabricator

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/authprofile"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
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
	profiles := authprofile.Commands{Label: "Phabricator", Selected: site, Backend: phabProfiles}
	cmd.AddCommand(newPhabricatorAuthLoginCmd(site))
	cmd.AddCommand(confirmProfileRemoval(commandmeta.Set(profiles.RemoveCmd(), commandmeta.Hints{
		HighRisk:        true,
		ToolAnnotations: commandmeta.ToolAnnotations{DestructiveHint: true},
		DataAccess:      []commandmeta.DataAccess{commandmeta.DataAccountMetadata},
	}), site))
	cmd.AddCommand(newPhabricatorAuthStatusCmd(site))
	cmd.AddCommand(commandmeta.Set(profiles.ProfilesCmd(), commandmeta.Hints{
		ToolAnnotations: commandmeta.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
		DataAccess:      []commandmeta.DataAccess{commandmeta.DataAccountMetadata},
	}))
	cmd.AddCommand(commandmeta.Set(profiles.DefaultCmd(), commandmeta.Hints{
		ToolAnnotations: commandmeta.ToolAnnotations{DestructiveHint: true, IdempotentHint: true},
		DataAccess:      []commandmeta.DataAccess{commandmeta.DataAccountMetadata},
	}))
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
