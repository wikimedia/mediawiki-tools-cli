package gerrit

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/authprofile"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

var gerritProfiles = authprofile.Manager{Service: "gerrit", Label: "Gerrit", SecretField: "password", Endpoint: secrets.GerritEndpoint}

func gerritProfileCredential(name string) config.Credential {
	return config.Credential{Key: gerritProfiles.Prefix(name) + ".password", Service: "gerrit", Endpoint: secrets.GerritEndpoint, Account: config.State().EffectiveKoanf.String(gerritProfiles.Prefix(name) + ".username")}
}

func selectedGerritProfile(cmd *cobra.Command) (string, error) {
	name, _ := cmd.Flags().GetString("profile")
	return gerritProfiles.Selected(strings.TrimSpace(name))
}

func requireGerritProfileName(name string) error {
	if !authprofile.ValidName(name) {
		return fmt.Errorf("profile must contain only letters, digits, underscores or hyphens")
	}
	return nil
}
