package cmd

import (
	_ "embed"
	"os/exec"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/cli"
)

//go:embed long/mwdd_gerrit.md
var gerritLong string

func NewGerritCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "gerrit",
		Short: "Wikimedia Gerrit",
		Long:  cli.RenderMarkdown(gerritLong),
		RunE:  nil,
	}
}

// TODO factor this into a nice package / util?
func sshGerritCommand(args []string) *exec.Cmd {
	ssh := exec.Command("ssh", "-p", "29418", "gerrit.wikimedia.org", "gerrit")
	ssh.Args = append(ssh.Args, args...)
	return ssh
}

func gerritAttachToCmd(rootCmd *cobra.Command) {
	gerritCmd := NewGerritCmd()
	rootCmd.AddCommand(gerritCmd)

	gerritChangesCmd := NewGerritChangesCmd()
	gerritCmd.AddCommand(gerritChangesCmd)
	gerritChangesListCmd := NewGerritChangesListCmd()
	gerritChangesCmd.AddCommand(gerritChangesListCmd)

	gerritGroupCmd := NewGerritGroupCmd()
	gerritCmd.AddCommand(gerritGroupCmd)
	gerritGroupCmd.AddCommand(NewGerritGroupListCmd())
	gerritGroupCmd.AddCommand(NewGerritGroupSearchCmd())
	gerritGroupCmd.AddCommand(NewGerritGroupMembersCmd())

	gerritProjectCmd := NewGerritProjectCmd()
	gerritCmd.AddCommand(gerritProjectCmd)
	gerritProjectCmd.AddCommand(NewGerritProjectListCmd())
	gerritProjectCmd.AddCommand(NewGerritProjectSearchCmd())
	gerritProjectCmd.AddCommand(NewGerritProjectCurrentCmd())
}
