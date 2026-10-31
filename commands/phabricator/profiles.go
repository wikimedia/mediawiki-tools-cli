package phabricator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	mwconfig "gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func validProfileName(name string) bool {
	if name == "" {
		return false
	}
	for _, c := range name {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

// Profile discovery only reads public configuration metadata, never credentials.
func savedProfiles() []string {
	names := map[string]bool{}
	state := mwconfig.State()
	for _, conf := range []map[string]interface{}{state.OnDiskKoanf.Raw(), state.EffectiveKoanf.Raw()} {
		phab, _ := conf["phabricator"].(map[string]interface{})
		sites, _ := phab["profiles"].(map[string]interface{})
		for name := range sites {
			names[name] = true
		}
	}
	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

func completeProfiles(_ *cobra.Command, _ []string, prefix string) ([]string, cobra.ShellCompDirective) {
	var result []string
	for _, name := range savedProfiles() {
		if strings.HasPrefix(name, prefix) {
			result = append(result, name)
		}
	}
	return result, cobra.ShellCompDirectiveNoFileComp
}

func newPhabricatorAuthListCmd() *cobra.Command {
	return &cobra.Command{
		Use: "profiles", Aliases: []string{"list"}, Short: "List saved Phabricator account profiles without retrieving tokens", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			profiles := savedProfiles()
			if len(profiles) == 0 {
				cmd.Println("No saved Phabricator profiles. Use phabricator auth login --profile <name>.")
				return nil
			}
			cmd.Println("Saved Phabricator profiles:")
			state := mwconfig.State()
			for _, name := range profiles {
				prefix := "phabricator.profiles." + name + "."
				username := state.EffectiveKoanf.String(prefix + "username")
				endpoint := state.EffectiveKoanf.String(prefix + "url")
				marker := ""
				if name == defaultProfile() {
					marker = " (default)"
				}
				if state.EffectiveKoanf.Bool(prefix + "logged_out") {
					marker += " (logged out)"
				}
				cmd.Println(fmt.Sprintf("  %s%s — %s — %s", name, marker, username, endpoint))
			}
			return nil
		},
	}
}
