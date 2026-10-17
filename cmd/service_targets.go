package cmd

import (
	"net/url"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// serviceTargets maps convenient service names and service entrypoint hosts to
// the existing top-level command which owns that service's command tree.
//
// A target host identifies a service context; it does not necessarily mean
// every command makes HTTP requests directly to that host. For example, the
// Cloud VPS command uses OpenStack's API endpoint while Horizon is its web UI.
var serviceTargets = map[string]string{
	"@codesearch":                          "codesearch",
	"@codesearch.wmcloud.org":              "codesearch",
	"@cloudvps":                            "cloudvps",
	"@horizon.wikimedia.org":               "cloudvps",
	"@openstack.eqiad1.wikimediacloud.org": "cloudvps",
	"@gerr":                                "gerrit",
	"@gerrit.wikimedia.org":                "gerrit",
	"@gitlab":                              "gitlab",
	"@gitlab.wikimedia.org":                "gitlab",
	"@phab":                                "phabricator",
	"@phabricator.wikimedia.org":           "phabricator",
	"@toolhub":                             "toolhub",
	"@toolhub.wikimedia.org":               "toolhub",
	"@toolforge":                           "tools",
	"@toolforge.org":                       "tools",
	"@login.toolforge.org":                 "tools",
	"@tools.wmflabs.org":                   "tools",
	"@en.wikipedia.org":                    "wiki",
	"@commons.wikimedia.org":               "wiki",
	"@www.wikidata.org":                    "wiki",
	"@meta.wikimedia.org":                  "wiki",
	"@test.wikipedia.org":                  "wiki",
	"@www.mediawiki.org":                   "wiki",
}

// completeServiceTargets offers canonical service hostnames for the @target
// prefix. Cobra's normal command completion does not include command aliases,
// so the target aliases are completed explicitly here.
func completeServiceTargets(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !strings.HasPrefix(toComplete, "@") {
		return nil, cobra.ShellCompDirectiveDefault
	}

	unique := make(map[string]struct{}, len(serviceTargets))
	for target := range serviceTargets {
		if strings.Count(target, ".") == 0 {
			continue
		}
		if strings.HasPrefix(target, toComplete) {
			unique[target] = struct{}{}
		}
	}

	completions := make([]string, 0, len(unique))
	for target := range unique {
		completions = append(completions, target)
	}
	sort.Strings(completions)
	return completions, cobra.ShellCompDirectiveNoFileComp
}

// resolveServiceTargetArgs rewrites a leading @target to its owning command.
// When called for Cobra's __complete command, it rewrites the completed target
// token too, so subsequent completion shows that service's actual subcommands.
func resolveServiceTargetArgs(args []string) []string {
	if len(args) == 0 {
		return args
	}

	index := 0
	if args[0] == "__complete" || args[0] == "__completeNoDesc" {
		index = 1
	}
	if index >= len(args) || !strings.HasPrefix(args[index], "@") {
		return args
	}

	target := args[index]
	if command, ok := serviceTargets[target]; ok {
		args[index] = command
		if command == "wiki" {
			host := strings.TrimPrefix(target, "@")
			args = append(args[:index+1], append([]string{"--wiki=https://" + host + "/w/api.php"}, args[index+1:]...)...)
		}
		return args
	}

	// Toolforge web services use one hostname per tool. Route those targets to
	// the corresponding tool's exec/cp command where possible.
	host := strings.TrimPrefix(target, "@")
	if strings.HasSuffix(host, ".toolforge.org") {
		tool := strings.TrimSuffix(host, ".toolforge.org")
		if tool != "" && !strings.Contains(tool, ".") {
			args[index] = "tools"
			if index+1 < len(args) && (args[index+1] == "exec" || args[index+1] == "cp") {
				args = append(args[:index+2], append([]string{"--tool=" + tool}, args[index+2:]...)...)
			}
			return args
		}
	}
	if host == "tools.wmflabs.org" {
		args[index] = "tools"
		return args
	}

	// A bare hostname can be used as a generic MediaWiki site target. Keep
	// aliases without a dot as errors rather than guessing their meaning.
	parsed, err := url.Parse("https://" + host)
	if err != nil || parsed.Host != host || !strings.Contains(host, ".") || strings.ContainsAny(host, " /\\") {
		return args
	}

	args[index] = "wiki"
	args = append(args[:index+1], append([]string{"--wiki=https://" + host + "/w/api.php"}, args[index+1:]...)...)
	return args
}
