package ziki

import (
	"bufio"
	_ "embed"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	"golang.org/x/term"
)

//go:embed README.md
var zikiReadme string

func Cmd() *cobra.Command {
	var scheme string
	var noClear bool
	var levelNumber int
	cmd := &cobra.Command{
		Use:   "ziki",
		Short: "Play a Wikimedia release adventure",
		Long:  cli.RenderMarkdown(zikiReadme),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if scheme != "dark" && scheme != "light" && scheme != "none" {
				return fmt.Errorf("--color must be dark, light or none")
			}
			if levelNumber < 1 || levelNumber > len(campaign) {
				return fmt.Errorf("--level must be between 1 and %d", len(campaign))
			}
			clearScreen := false
			if file, ok := cmd.OutOrStdout().(*os.File); ok {
				clearScreen = !noClear && os.Getenv("TERM") != "dumb" && term.IsTerminal(int(file.Fd()))
			}
			game := Game{level: levelNumber - 1, clearScreen: clearScreen, ColorScheme: scheme, input: bufio.NewScanner(cmd.InOrStdin()), writer: cmd.OutOrStdout()}
			game.readInteractive = interactiveReader(cmd.InOrStdin(), cmd.OutOrStdout())
			return game.Play()
		},
		Hidden: true,
	}
	cmd.Flags().StringVar(&scheme, "color", "dark", "Colour scheme: dark, light or none")
	cmd.Flags().BoolVar(&noClear, "no-clear", false, "Keep the scrolling transcript instead of clearing the terminal")
	cmd.Flags().IntVar(&levelNumber, "level", 1, "Starting level (1–16; useful for trying later assignments)")
	return cmd
}
