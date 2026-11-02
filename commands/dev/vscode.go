package dev

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devenv"
	"gitlab.wikimedia.org/repos/releng/cli/internal/devremote"
)

func NewVSCodeCmd() *cobra.Command {
	command := &cobra.Command{Use: "vscode", Aliases: []string{"code-server"}, GroupID: "service", Short: "Manage the remote browser editor", Long: "Run code-server on the VM with access to the context's files and tools. The service is authenticated and bound to loopback; remote open --editor web supplies browser access over SSH."}
	for _, action := range []string{"start", "stop", "status"} {
		action := action
		child := &cobra.Command{Use: action, Short: fmt.Sprintf("%s the browser editor", action), Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			workspace, metadata, source := os.Getenv("MW_REMOTE_WORKSPACE"), os.Getenv("MW_REMOTE_METADATA"), os.Getenv("MW_REMOTE_SOURCE")
			if workspace == "" || metadata == "" || source == "" {
				return fmt.Errorf("select a remote context, or run from its remote terminal")
			}
			return devremote.WebEditor(cmd.Context(), workspace, metadata, source, action, devenv.Context, cmd.OutOrStdout(), cmd.ErrOrStderr())
		}}
		if action == "start" {
			child.Aliases = []string{"create"}
		}
		command.AddCommand(child)
	}
	return command
}
