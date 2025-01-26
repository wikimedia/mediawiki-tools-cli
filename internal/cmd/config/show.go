package config

import (
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func NewConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Shows the raw config",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			config.LoadFromDisk().PrettyPrint()
		},
	}
}
