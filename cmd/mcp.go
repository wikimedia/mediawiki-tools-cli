package cmd

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/commands/phabricator"
	"gitlab.wikimedia.org/repos/releng/cli/commands/wiki"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mcpserver"
)

func newMCPCmd() *cobra.Command {
	var services, deferredServices, profileBindings []string
	annotations := commandmeta.Annotations(mcpserver.ReadHints(commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata))
	annotations[cli.SkipConfigAnnotation] = "true"
	cmd := &cobra.Command{
		Use:         "mcp",
		Short:       "Serve selected Wikimedia services over MCP",
		Long:        "Start a local stdio MCP server. With no flags, serve anonymous Phabricator search and public wiki page metadata. Select initially visible services with --services and authorize on-demand services with --deferred-services. To use a named Phabricator account, bind it at startup with --profile phab=NAME.",
		Example:     "mcp --services phab,wiki\nmcp --services phab --deferred-services wiki\nmcp --services phab --profile phab=work",
		Args:        cobra.NoArgs,
		Annotations: annotations,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !cmd.Flags().Changed("services") {
				services = slices.DeleteFunc(services, func(name string) bool {
					return slices.Contains(deferredServices, name)
				})
			}
			profile, err := mcpPhabricatorProfile(services, deferredServices, profileBindings)
			if err != nil {
				return err
			}
			groups := make([]mcpserver.Service, 0, len(services)+len(deferredServices))
			for _, selection := range []struct {
				names    []string
				deferred bool
			}{{services, false}, {deferredServices, true}} {
				for _, name := range selection.names {
					group := mcpserver.Service{Name: name, Deferred: selection.deferred}
					switch name {
					case "phab":
						group.Tools, err = phabricator.MCPTools(cmd, profile)
					case "wiki":
						group.Tools = wiki.MCPTools()
					}
					if err != nil {
						return err
					}
					groups = append(groups, group)
				}
			}
			server := mcpserver.New("mw")
			mcpserver.RegisterCatalog(server, groups)
			return mcpserver.Serve(cmd.Context(), server)
		},
	}
	cmd.Flags().StringSliceVar(&services, "services", []string{"phab", "wiki"}, "Comma-separated MCP services to expose immediately (phab,wiki)")
	cmd.Flags().StringSliceVar(&deferredServices, "deferred-services", nil, "Comma-separated MCP services eligible for on-demand activation (phab,wiki)")
	cmd.Flags().StringArrayVar(&profileBindings, "profile", nil, "Bind a saved profile to a service as SERVICE=NAME (currently phab only)")
	cmd.Flags().Bool("yes", false, "Assert user approval if a profiled service requires it")
	return cmd
}

func mcpPhabricatorProfile(services, deferred, bindings []string) (string, error) {
	if len(services)+len(deferred) == 0 {
		return "", fmt.Errorf("select at least one MCP service")
	}
	enabled := make(map[string]bool, len(services)+len(deferred))
	for _, names := range [][]string{services, deferred} {
		for _, name := range names {
			switch name {
			case "phab", "wiki":
			default:
				return "", fmt.Errorf("unknown MCP service %q; choose phab or wiki", name)
			}
			if enabled[name] {
				return "", fmt.Errorf("MCP service %q was selected more than once", name)
			}
			enabled[name] = true
		}
	}
	var profile string
	for _, binding := range bindings {
		service, name, ok := strings.Cut(binding, "=")
		if !ok || strings.TrimSpace(name) == "" {
			return "", fmt.Errorf("use --profile SERVICE=NAME, for example --profile phab=work")
		}
		if !enabled[service] {
			return "", fmt.Errorf("profile binding requires MCP service %q to be authorized at launch", service)
		}
		if service != "phab" {
			return "", fmt.Errorf("MCP service %q does not support profiles", service)
		}
		if profile != "" {
			return "", fmt.Errorf("only one profile may be bound to the Phabricator MCP service")
		}
		profile = strings.TrimSpace(name)
	}
	return profile, nil
}
