package wiki

import (
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func chooseOAuthScopes(defaults []string) ([]string, error) {
	var scopes []string
	err := survey.AskOne(&oauthScopePrompt{selection: newOAuthSelection(defaults)}, &scopes)
	return scopes, err
}

func requestedOAuthScopes(cmd *cobra.Command, clientID string, explicit []string, all, interactive bool, previous *config.WikiProfile, choose func([]string) ([]string, error)) ([]string, error) {
	if cmd.Flags().Changed("scope") {
		return explicit, nil
	}
	if all {
		scopes := make([]string, 0, len(defaultOAuthGrants))
		for _, grant := range defaultOAuthGrants {
			scopes = append(scopes, grant.Scope)
		}
		return scopes, nil
	}
	if interactive {
		var defaults []string
		if previous != nil && previous.ClientID == clientID {
			defaults = strings.Fields(previous.RequestedScope)
		}
		scopes, err := choose(defaults)
		if err == nil && len(scopes) == 0 {
			scopes = []string{"basic"}
		}
		return scopes, err
	}
	return []string{"basic"}, nil
}

func completeOAuthScopes(_ *cobra.Command, _ []string, prefix string) ([]string, cobra.ShellCompDirective) {
	var matches []string
	for _, grant := range defaultOAuthGrants {
		if strings.HasPrefix(grant.Scope, prefix) {
			matches = append(matches, grant.Scope+"\t"+grant.Label+" ["+grant.Risk+"]")
		}
	}
	return matches, cobra.ShellCompDirectiveNoFileComp
}
