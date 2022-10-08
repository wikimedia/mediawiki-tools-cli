package ziki

import (
	_ "embed"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

//go:embed README.md
var zikiReadme string

func NewZikiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ziki",
		Short: "Text-based game",
		Long:  cli.RenderMarkdown(zikiReadme),
		Run: func(cmd *cobra.Command, args []string) {
			game := *new(Game)
			game.Play()
		},
		Hidden: true,
	}
	return cmd
}
