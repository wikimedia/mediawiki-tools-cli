package config

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func NewConfigShowCmd() *cobra.Command {
	var effective, showSecrets bool

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Shows the config",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s := config.State()
			values := s.OnDiskKoanf
			if effective {
				values = s.EffectiveKoanf
			}
			if !showSecrets {
				values = config.Redacted(values)
			}
			text, err := config.PrettyPrint(values)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), text)
			return err
		},
	}

	cmd.Flags().BoolVar(&effective, "effective", false, "Show the effective config, considering all sources.")

	cmd.Flags().BoolVar(&showSecrets, "show-secrets", false, "Reveal plaintext secrets in the configuration")
	return cmd
}
