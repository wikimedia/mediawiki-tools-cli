package config

import (
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func NewConfigShowCmd() *cobra.Command {
	var effective, showSecrets bool

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Shows the config",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			s := config.State()
			values := s.OnDiskKoanf
			if effective {
				values = s.EffectiveKoanf
			}
			if !showSecrets {
				values = config.Redacted(values)
			}
			cmd.Printf("%s\n", config.PrettyPrint(values))
		},
	}

	cmd.Flags().BoolVar(&effective, "effective", false, "Show the effective config, considering all sources.")

	cmd.Flags().BoolVar(&showSecrets, "show-secrets", false, "Reveal plaintext secrets in the configuration")
	return cmd
}
