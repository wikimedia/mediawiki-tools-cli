package cmd

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/commands/gerrit"
	"gitlab.wikimedia.org/repos/releng/cli/commands/gitlab"
	"gitlab.wikimedia.org/repos/releng/cli/commands/phabricator"
	"gitlab.wikimedia.org/repos/releng/cli/commands/wiki"
	"gitlab.wikimedia.org/repos/releng/cli/internal/accesspolicy"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mcpserver"
)

var publicMCPServices = map[string]func() []mcpserver.Operation{
	"wiki":   wiki.MCPTools,
	"gerrit": gerrit.MCPTools,
	"gitlab": gitlab.MCPTools,
}

func newMCPCmd() *cobra.Command {
	var services, deferredServices, profileBindings []string
	defaultServices := []string{"phab", "gerrit", "gitlab", "wiki"}
	serviceList := strings.Join(append([]string{"phab"}, slices.Sorted(maps.Keys(publicMCPServices))...), ",")
	annotations := commandmeta.Annotations(mcpserver.ReadHints(commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata))
	annotations[cli.SkipConfigAnnotation] = "true"
	cmd := &cobra.Command{
		Use:         "mcp",
		Short:       "Serve selected Wikimedia services over MCP",
		Long:        "Start a local stdio MCP server for public Phabricator, Gerrit, GitLab, and wiki reads. Select services with --services or --deferred-services. Bind --profile phab=NAME for account-backed search. Public-only access is the default. Add --public-only=false and --allow-paste to expose account-visible paste content.",
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
			if allowed, _ := cmd.Flags().GetBool(phabricator.MCPAllowPasteFlag); allowed && profile == "" {
				return fmt.Errorf("--allow-paste requires an explicit Phabricator profile")
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
					default:
						group.Tools = publicMCPServices[name]()
					}
					if err != nil {
						return err
					}
					groups = append(groups, group)
				}
			}
			server := mcpserver.New("mw")
			if err := mcpserver.RegisterCatalog(server, groups); err != nil {
				return err
			}
			return mcpserver.Serve(cmd.Context(), server)
		},
	}
	accesspolicy.Support(cmd)
	cmd.Flags().StringSliceVar(&services, "services", defaultServices, "Comma-separated MCP services to expose immediately ("+serviceList+")")
	cmd.Flags().StringSliceVar(&deferredServices, "deferred-services", nil, "Comma-separated MCP services eligible for on-demand activation ("+serviceList+")")
	cmd.Flags().StringArrayVar(&profileBindings, "profile", nil, "Bind a saved profile to a service as SERVICE=NAME (currently phab only)")
	cmd.Flags().Bool("yes", false, "Assert user approval if a profiled service requires it")
	cmd.Flags().Bool(phabricator.MCPAllowPasteFlag, false, "Expose account-visible pastes with a Phabricator profile and --public-only=false")
	return cmd
}

// MCPCommandTools describes the CLI tools that can be registered at launch.
func MCPCommandTools() []mcpserver.CommandTool {
	tools := phabricator.MCPCommandTools()
	for _, service := range slices.Sorted(maps.Keys(publicMCPServices)) {
		for _, operation := range publicMCPServices[service]() {
			tools = append(tools, operation.CommandTool())
		}
	}
	return tools
}

func mcpPhabricatorProfile(services, deferred, bindings []string) (string, error) {
	if len(services)+len(deferred) == 0 {
		return "", fmt.Errorf("select at least one MCP service")
	}
	enabled := make(map[string]bool, len(services)+len(deferred))
	for _, names := range [][]string{services, deferred} {
		for _, name := range names {
			if name != "phab" && publicMCPServices[name] == nil {
				return "", fmt.Errorf("unknown MCP service %q", name)
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
