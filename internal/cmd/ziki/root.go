package ziki

import (
	"github.com/spf13/cobra"
)

func NewZikiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ziki",
		Short: "Text-based game",
		Run: func(cmd *cobra.Command, args []string) {
			game := *new(Game)
			game.Play()
		},
		Hidden: true,
	}
	return cmd
}
