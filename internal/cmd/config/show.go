package config

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func NewConfigShowCmd() *cobra.Command {
	var effective bool

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Shows the config",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			s := config.State()
			if effective {
				fmt.Printf("%s\n", config.PrettyPrint(s.EffectiveKoanf))
			} else {
				fmt.Printf("%s\n", config.PrettyPrint(s.OnDiskKoanf))
			}
		},
	}

	cmd.Flags().BoolVar(&effective, "effective", false, "Show the effective config, considering all sources.")

	return cmd
}
