package gerrit

import (
	_ "embed"
	"os/exec"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	sshutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/ssh"
)

//go:embed gerrit.long.md
var gerritLong string

func NewGerritCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gerrit",
		Short: "Interact with the Wikimedia Gerrit instance (WORK IN PROGRESS)",
		Long:  cli.RenderMarkdown(gerritLong),
		RunE:  nil,
	}

	cmd.Annotations = make(map[string]string)
	cmd.Annotations["group"] = "Service"

	cmd.AddCommand(NewGerritAPICmd())
	cmd.AddCommand(NewGerritSSHCmd())
	cmd.AddCommand(NewGerritChangesCmd())
	cmd.AddCommand(NewGerritGroupCmd())
	cmd.AddCommand(NewGerritProjectCmd())
	cmd.AddCommand(NewGerritAuthCmd())

	return cmd
}

func sshGerritCommand(args []string) *exec.Cmd {
	return sshutil.CommandOnSSHHost("gerrit.wikimedia.org", "29418", append([]string{"gerrit"}, args...))
}
