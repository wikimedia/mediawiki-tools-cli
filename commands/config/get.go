package config

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func NewConfigGetCmd() *cobra.Command {
	var showSecrets bool
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Gets a settings from the config",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			keyName := args[0]

			k := config.State().OnDiskKoanf
			if !showSecrets {
				k = config.Redacted(k)
			}
			v := k.Get(keyName)

			_, err := fmt.Fprintln(cmd.OutOrStdout(), v)
			return err
		},
	}
	cmd.Flags().BoolVar(&showSecrets, "show-secrets", false, "Reveal a plaintext secret in the configuration")
	return cmd
}
