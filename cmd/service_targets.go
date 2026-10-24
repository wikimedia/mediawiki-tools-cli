package cmd

import (
	"io"
	"net/url"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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

// siteTargetAliases name explicit site shortcuts; unknown names are never
// expanded heuristically into a different domain.
var siteTargetAliases = map[string]string{
	"@en.wiki": "@en.wikipedia.org",
}

// completeServiceTargets offers canonical service hostnames for the @target
// prefix. Cobra's normal command completion does not include command aliases,
// so the target aliases are completed explicitly here.
func completeServiceTargets(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 || !strings.HasPrefix(toComplete, "@") {
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

// completeTargetHelp extends command-reference completion without changing
// workflow-guide completion or Cobra's normal help behavior.
func completeTargetHelp(root *cobra.Command) {
	root.InitDefaultHelpCmd()
	help, _, err := root.Find([]string{"help"})
	if err != nil {
		return
	}
	original := help.ValidArgsFunction
	help.ValidArgsFunction = func(cmd *cobra.Command, args []string, partial string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 && strings.HasPrefix(partial, "@") {
			return completeServiceTargets(cmd, args, partial)
		}
		if original != nil {
			return original(cmd, args, partial)
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

// targetFlagValue lets pflag recognize arity and shorthand clusters without
// setting real options or marking flags changed before Cobra parses them.
type targetFlagValue struct{ pflag.Value }

func (targetFlagValue) Set(string) error { return nil }

// firstTargetArgument locates the first positional argument after recognized
// flags, leaving validation and actual option assignment to Cobra.
func firstTargetArgument(cmd *cobra.Command, args []string, start int) int {
	if start >= len(args) {
		return -1
	}
	flags := pflag.NewFlagSet("service target", pflag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.SetInterspersed(false)
	for _, set := range []*pflag.FlagSet{cmd.InheritedFlags(), cmd.PersistentFlags(), cmd.Flags()} {
		set.VisitAll(func(flag *pflag.Flag) {
			if flags.Lookup(flag.Name) != nil {
				return
			}
			cloned := *flag
			cloned.Value = targetFlagValue{flag.Value}
			flags.AddFlag(&cloned)
		})
	}
	if err := flags.Parse(args[start:]); err != nil || flags.ArgsLenAtDash() >= 0 || flags.NArg() == 0 {
		return -1
	}
	return len(args) - flags.NArg()
}

func insertTargetFlags(args []string, index int, flags ...string) []string {
	result := make([]string, 0, len(args)+len(flags))
	result = append(result, args[:index]...)
	result = append(result, flags...)
	return append(result, args[index:]...)
}

// resolveServiceTargetArgs selects an existing command tree after global flags.
// The last completion token must remain untouched: it is a partial target, not
// a completed command. Neither argv nor real flag values are modified here.
func resolveServiceTargetArgs(root *cobra.Command, args []string) []string {
	index := firstTargetArgument(root, args, 0)
	if index < 0 {
		return args
	}
	completing := args[index] == "__complete" || args[index] == "__completeNoDesc"
	if completing {
		index = firstTargetArgument(root, args, index+1)
	}
	if index < 0 {
		return args
	}
	helping := args[index] == "help"
	if helping {
		index = firstTargetArgument(root, args, index+1)
	}
	if index < 0 || !strings.HasPrefix(args[index], "@") || (completing && index == len(args)-1) {
		return args
	}

	target := args[index]
	if canonical, ok := siteTargetAliases[target]; ok {
		target = canonical
	}
	command, known := serviceTargets[target]
	host := strings.TrimPrefix(target, "@")
	if (command == "wiki" || !known) && isWikimediaWikiHost(host) {
		root.AddCommand(newWikiTargetCmd(host))
		result := append([]string(nil), args...)
		result[index] = "@" + host
		return result
	}
	var flags []string
	if known && command == "wiki" {
		flags = []string{"--wiki=https://" + strings.TrimPrefix(target, "@") + "/w/api.php"}
	} else if !known {
		if strings.HasSuffix(host, ".toolforge.org") {
			tool := strings.TrimSuffix(host, ".toolforge.org")
			if tool != "" && !strings.Contains(tool, ".") {
				command = "tools"
				if !helping {
					for _, child := range root.Commands() {
						if child.Name() != command {
							continue
						}
						sub := firstTargetArgument(child, args, index+1)
						if sub >= 0 && (args[sub] == "exec" || args[sub] == "cp") {
							result := append([]string(nil), args...)
							result[index] = command
							return insertTargetFlags(result, sub+1, "--tool="+tool)
						}
					}
				}
			}
		}
		if command == "" {
			// A generic site remains an explicit hostname, with the existing /w/api.php
			// convention. This does not discover arbitrary website capabilities.
			parsed, err := url.Parse("https://" + host)
			if err != nil || parsed.Host != host || !strings.Contains(host, ".") || strings.ContainsAny(host, " /\\") {
				return args
			}
			command = "wiki"
			flags = []string{"--wiki=https://" + host + "/w/api.php"}
		}
	}
	result := append([]string(nil), args...)
	result[index] = command
	if helping {
		return result
	}
	return insertTargetFlags(result, index+1, flags...)
}
