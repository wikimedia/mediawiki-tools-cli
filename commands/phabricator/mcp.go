package phabricator

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mcpserver"
)

func newPhabricatorMCPCmd(site *string) *cobra.Command {
	annotations := commandmeta.Annotations(mcpserver.ReadHints(commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata))
	annotations[cli.SkipConfigAnnotation] = "true"
	return &cobra.Command{
		Use:         "mcp",
		Short:       "Serve Phabricator task search over MCP",
		Long:        "Start a local stdio MCP server that searches Phabricator tasks. Reads are anonymous unless you select a saved account with --profile.",
		Example:     "mcp\nmcp --profile personal",
		Annotations: annotations,
		Args:        cobra.NoArgs,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if (cmd.Flags().Changed("profile") || cmd.Flags().Changed("site")) && strings.TrimSpace(*site) == "" {
				return fmt.Errorf("provide a saved Phabricator profile name")
			}
			if commandFlagEnabled(cmd, "allow-non-public") && *site == "" {
				return fmt.Errorf("--allow-non-public requires an explicit Phabricator profile")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			server := mcpserver.New("mw-phabricator")
			tools, err := MCPTools(cmd, *site)
			if err != nil {
				return err
			}
			for _, tool := range tools {
				tool.Register(server)
			}
			return mcpserver.Serve(cmd.Context(), server)
		},
	}
}

// MCPTools binds the handlers to anonymous access or a named account at startup.
// Anonymous tools do not load the CLI configuration.
func MCPTools(cmd *cobra.Command, profile string) ([]mcpserver.Operation, error) {
	if profile == "" {
		return []mcpserver.Operation{taskSearchTool(true, newAnonymousPhabricator().searchTasks)}, nil
	}
	if err := checkPhabricatorAgentPolicy(cmd); err != nil {
		return nil, err
	}

	selected, err := phabProfiles.Selected(strings.TrimSpace(profile))
	if err != nil {
		return nil, err
	}
	client, _, err := loadCommandClient(cmd, selected, false)
	if err != nil {
		return nil, err
	}
	return []mcpserver.Operation{taskSearchTool(false, limitedProfileTaskSearch(client))}, nil
}
