package ziki

import (
	"bufio"
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

//go:embed README.md
var zikiReadme string

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ziki",
		Short: "Text-based game",
		Long:  cli.RenderMarkdown(zikiReadme),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			game := Game{input: bufio.NewScanner(cmd.InOrStdin()), writer: cmd.OutOrStdout()}
			return game.Play()
		},
		Hidden: true,
	}
	return cmd
}
