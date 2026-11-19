package phabricator

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/accesspolicy"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	"gitlab.wikimedia.org/repos/releng/cli/internal/mcpserver"
)

const MCPAllowPasteFlag = "allow-paste"

func newPhabricatorMCPCmd(site *string) *cobra.Command {
	annotations := commandmeta.Annotations(mcpserver.ReadHints(commandmeta.DataPrivateContent, commandmeta.DataPrivateMetadata))
	annotations[cli.SkipConfigAnnotation] = "true"
	cmd := &cobra.Command{
		Use:         "mcp",
		Short:       "Serve Phabricator reads over MCP",
		Long:        "Start a local stdio MCP server. Task and comment details are public. Bind --profile to search with an account. Public-only access is the default. Add --public-only=false and --allow-paste to expose account-visible paste content.",
		Example:     "mcp\nmcp --profile personal",
		Annotations: annotations,
		Args:        cobra.NoArgs,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if (cmd.Flags().Changed("profile") || cmd.Flags().Changed("site")) && strings.TrimSpace(*site) == "" {
				return fmt.Errorf("provide a saved Phabricator profile name")
			}
			if commandFlagEnabled(cmd, "allow-non-public") {
				return fmt.Errorf("use --public-only=false to allow account-visible MCP results")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			server := mcpserver.New("mw-phabricator")
			tools, err := MCPTools(cmd, *site)
			if err != nil {
				return err
			}
			if err := mcpserver.ValidateOperations(tools); err != nil {
				return err
			}
			for _, tool := range tools {
				tool.Register(server)
			}
			return mcpserver.Serve(cmd.Context(), server)
		},
	}
	accesspolicy.Support(cmd)
	cmd.Flags().Bool(MCPAllowPasteFlag, false, "Expose account-visible paste content using the explicit Phabricator profile")
	return cmd
}

// MCPTools binds the handlers to anonymous access or a named account at startup.
// Anonymous tools do not load the CLI configuration.
func MCPTools(cmd *cobra.Command, profile string) ([]mcpserver.Operation, error) {
	allowPaste := commandFlagEnabled(cmd, MCPAllowPasteFlag)
	if profile == "" {
		if allowPaste {
			return nil, fmt.Errorf("--allow-paste requires an explicit Phabricator profile")
		}
		return phabricatorMCPTools(nil, false), nil
	}
	publicOnly, err := accesspolicy.MCP(cmd)
	if err != nil {
		return nil, err
	}
	if publicOnly && allowPaste {
		return nil, fmt.Errorf("--allow-paste requires --public-only=false and an explicit Phabricator profile")
	}
	if err := checkPhabricatorAgentPolicy(cmd); err != nil {
		return nil, err
	}

	selected, err := phabProfiles.Selected(strings.TrimSpace(profile))
	if err != nil {
		return nil, err
	}
	client, _, err := loadClientWithAccess(cmd.Context(), selected, false, taskAccess{publicOnly: publicOnly})
	if err != nil {
		return nil, err
	}
	return phabricatorMCPTools(client, allowPaste), nil
}

// MCPCommandTools documents the public and account-backed tools without opening a profile.
func MCPCommandTools() []mcpserver.CommandTool {
	operations := phabricatorMCPTools(nil, true)
	profile := taskSearchTool(false, false, nil).CommandTool()
	tools := make([]mcpserver.CommandTool, 0, len(operations))
	for _, operation := range operations {
		tool := operation.CommandTool()
		if tool.Name == profile.Name {
			tool.Profile = &profile.ToolAccess
		}
		tools = append(tools, tool)
	}
	return tools
}

func phabricatorMCPTools(profile *conduitClient, allowPaste bool) []mcpserver.Operation {
	public := newAnonymousPhabricator()
	var search taskSearch = public.searchTasks
	if profile != nil {
		search = limitedProfileTaskSearch(profile)
	}
	tools := []mcpserver.Operation{
		taskSearchTool(profile == nil, profile == nil || profile.access.publicOnly, search),
		readTaskTool(public),
		readCommentsTool(public, "phabricator_get_comments"),
		readCommentsTool(public, "phabricator_get_task_comments"),
		projectViewTool(public),
	}
	if allowPaste {
		tools = append(tools, readPasteTool(profile))
	}
	return tools
}
