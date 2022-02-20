package gerrit

import (
	_ "embed"
	"os/exec"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

//go:embed long_gerrit.md
var gerritLong string

func NewGerritCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gerrit",
		Short: "Wikimedia Gerrit",
		Long:  cli.RenderMarkdown(gerritLong),
		RunE:  nil,
	}

	cmd.AddCommand(NewGerritAPICmd())
	cmd.AddCommand(NewGerritSSHCmd())
	cmd.AddCommand(NewGerritChangesCmd())
	cmd.AddCommand(NewGerritGroupCmd())
	cmd.AddCommand(NewGerritProjectCmd())

	return cmd
}

// TODO factor this into a nice package / util?
func sshGerritCommand(args []string) *exec.Cmd {
	ssh := exec.Command("ssh", "-p", "29418", "gerrit.wikimedia.org", "gerrit")
	ssh.Args = append(ssh.Args, args...)
	return ssh
}
