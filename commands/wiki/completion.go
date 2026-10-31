package wiki

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/wikiauth"
)

func registerWikiCompletions(cmd *cobra.Command) {
	_ = cmd.RegisterFlagCompletionFunc("profile", completeWikiProfiles)
	_ = cmd.RegisterFlagCompletionFunc("account", completeWikiProfiles)
	_ = cmd.RegisterFlagCompletionFunc("realm", completeWikiRealms)
	_ = cmd.RegisterFlagCompletionFunc("wiki", completeWikiSites)
}

// Completion uses public configuration and bundled sites only, never tokens,
// password-manager access, or live API discovery.
func completionProfiles() []config.WikiProfile {
	var accounts []config.WikiProfile
	for _, realm := range config.State().OnDiskKoanf.MapKeys("wiki.profiles") {
		for _, name := range config.State().OnDiskKoanf.MapKeys("wiki.profiles." + realm) {
			if account, err := config.LoadWikiProfile(realm, name); err == nil {
				accounts = append(accounts, account)
			}
		}
	}
	return accounts
}

func completionRealm(cmd *cobra.Command) string {
	if realm := cmd.Flag("realm").Value.String(); realm != "" {
		return realm
	}
	if account := cmd.Flag("profile").Value.String(); strings.Contains(account, "/") {
		realm, _, _ := strings.Cut(account, "/")
		return realm
	}
	if site := cmd.Flag("wiki").Value.String(); site != "" {
		if endpoint, err := wikiauth.Endpoint(site); err == nil {
			realm, err := realmForWiki(endpoint)
			if err == nil && realm != "" {
				return realm
			}
		}
	}
	return wikiDefaultRealm()
}

func completeWikiProfiles(cmd *cobra.Command, _ []string, prefix string) ([]string, cobra.ShellCompDirective) {
	explicitRealm := cmd.Flag("realm").Value.String()
	implicitRealm := completionRealm(cmd)
	var candidates []string
	for _, account := range completionProfiles() {
		if explicitRealm != "" && account.Realm != explicitRealm {
			continue
		}
		qualified := account.Realm + "/" + account.Name
		name := qualified
		if !strings.Contains(prefix, "/") && (explicitRealm != "" || (prefix != "" && account.Realm == implicitRealm && strings.HasPrefix(account.Name, prefix))) {
			name = account.Name
		}
		if strings.HasPrefix(name, prefix) {
			candidates = append(candidates, name+"\t"+account.Username+" ("+account.Method+")")
		}
	}
	sort.Strings(candidates)
	return candidates, cobra.ShellCompDirectiveNoFileComp
}

func completeWikiRealms(_ *cobra.Command, _ []string, prefix string) ([]string, cobra.ShellCompDirective) {
	values := map[string]bool{"wikimedia": true}
	for _, path := range []string{"wiki.realms", "wiki.profiles"} {
		for _, realm := range config.State().OnDiskKoanf.MapKeys(path) {
			if wikiauth.ValidName(realm) {
				values[realm] = true
			}
		}
	}
	return matchingCompletions(values, prefix), cobra.ShellCompDirectiveNoFileComp
}

func completeWikiSites(cmd *cobra.Command, _ []string, prefix string) ([]string, cobra.ShellCompDirective) {
	// Defaults must not hide other farms when selecting a wiki. Restrict the
	// suggestions only when an account or realm was explicitly selected.
	realm := ""
	if cmd.Flag("realm").Value.String() != "" || cmd.Flag("profile").Value.String() != "" {
		realm = completionRealm(cmd)
	}
	endpoints := make(map[string]bool)
	if realm == "" || realm == "wikimedia" {
		for _, endpoint := range wikiauth.WikimediaEndpoints() {
			endpoints[endpoint] = true
		}
	}
	for _, name := range config.State().OnDiskKoanf.MapKeys("wiki.realms") {
		if realm == "" || name == realm {
			for _, endpoint := range config.WikiRealmEndpoints(name) {
				if normalized, err := wikiauth.Endpoint(endpoint); err == nil {
					endpoints[normalized] = true
				}
			}
		}
	}
	for _, account := range completionProfiles() {
		if (realm == "" || account.Realm == realm) && account.Allows(account.Wiki) {
			endpoints[account.Wiki] = true
		}
	}
	values := make(map[string]bool)
	var selected *config.WikiProfile
	if name := cmd.Flag("profile").Value.String(); name != "" {
		profile, err := selectedWikiProfile(name)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		selected = &profile
	}
	for endpoint := range endpoints {
		if selected != nil && !selected.Allows(endpoint) {
			continue
		}
		value := endpoint
		if prefix != "" && !strings.Contains(prefix, "://") {
			// Bare HTTPS hosts are accepted by --wiki. Keep HTTP explicit.
			value = strings.TrimPrefix(value, "https://")
		}
		values[value] = true
	}
	return matchingCompletions(values, prefix), cobra.ShellCompDirectiveNoFileComp
}

func matchingCompletions(values map[string]bool, prefix string) []string {
	var matches []string
	for value := range values {
		if strings.HasPrefix(value, prefix) {
			matches = append(matches, value)
		}
	}
	sort.Strings(matches)
	return matches
}
