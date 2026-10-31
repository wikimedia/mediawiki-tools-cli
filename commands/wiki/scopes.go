package wiki

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiauth"
)

type oauthGrant struct{ Scope, Label string }

// Keep this list tied to the registered a-wikimedia-cli consumer version.
// Custom clients use explicit --scope values or an explicit --all-scopes.
var defaultOAuthGrants = []oauthGrant{
	{"highvolume", "High-volume (bot) access"},
	{"editpage", "Edit existing pages"},
	{"createeditmovepage", "Create, edit, and move pages"},
	{"editprotected", "Edit protected pages (requires existing account rights)"},
	{"uploadfile", "Upload new files"},
	{"uploadeditmovefile", "Upload, replace, and move files"},
	{"patrol", "Patrol changes to pages"},
	{"rollback", "Rollback changes to pages"},
	{"viewmywatchlist", "View your watchlist"},
	{"editmywatchlist", "Edit your watchlist"},
}

func chooseOAuthScopes(defaults []string) ([]string, error) {
	var options, selected []string
	for _, grant := range defaultOAuthGrants {
		options = append(options, grant.Label)
		for _, scope := range defaults {
			if scope == grant.Scope {
				selected = append(selected, grant.Label)
			}
		}
	}
	var labels []string
	if err := survey.AskOne(&survey.MultiSelect{Message: "Additional Wikimedia access (basic access is always included; space selects, enter confirms):", Options: options, Default: selected}, &labels); err != nil {
		return nil, err
	}
	scopes := []string{"basic"}
	for _, grant := range defaultOAuthGrants {
		for _, label := range labels {
			if label == grant.Label {
				scopes = append(scopes, grant.Scope)
			}
		}
	}
	return scopes, nil
}

func requestedOAuthScopes(cmd *cobra.Command, clientID string, explicit []string, all, interactive bool, previous *config.WikiProfile, choose func([]string) ([]string, error)) ([]string, error) {
	if cmd.Flags().Changed("scope") {
		if clientID == wikiauth.DefaultClientID {
			supported := map[string]bool{"basic": true}
			for _, grant := range defaultOAuthGrants {
				supported[grant.Scope] = true
			}
			for _, scope := range explicit {
				if !supported[scope] {
					return nil, fmt.Errorf("scope %q is not registered for this client; use a client registered for that permission", scope)
				}
			}
		}
		return explicit, nil
	}
	if all {
		if clientID != wikiauth.DefaultClientID {
			return nil, nil
		}
		scopes := []string{"basic"}
		for _, grant := range defaultOAuthGrants {
			scopes = append(scopes, grant.Scope)
		}
		return scopes, nil
	}
	if interactive && clientID == wikiauth.DefaultClientID {
		var defaults []string
		if previous != nil && previous.ClientID == clientID {
			defaults = strings.Fields(previous.RequestedScope)
		}
		return choose(defaults)
	}
	return []string{"basic"}, nil
}

func completeOAuthScopes(_ *cobra.Command, _ []string, prefix string) ([]string, cobra.ShellCompDirective) {
	scopes := []string{"basic\tBasic identity and read access"}
	for _, grant := range defaultOAuthGrants {
		scopes = append(scopes, grant.Scope+"\t"+grant.Label)
	}
	var matches []string
	for _, scope := range scopes {
		if strings.HasPrefix(scope, prefix) {
			matches = append(matches, scope)
		}
	}
	return matches, cobra.ShellCompDirectiveNoFileComp
}
