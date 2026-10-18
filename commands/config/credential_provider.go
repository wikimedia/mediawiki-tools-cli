package config

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func NewCredentialProviderCmd() *cobra.Command {
	return &cobra.Command{
		Use:       "provider [keyring|keepassxc|plaintext]",
		Short:     "Show or select the default credential provider",
		Long:      "The default applies to new credentials. Changing it does not move saved credentials; use migrate or transfer explicitly. Use --provider to override the default for a new credential.",
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []string{"keyring", "keepassxc", "plaintext"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				provider, err := config.DefaultCredentialProvider()
				if err != nil {
					return err
				}
				if provider == "" {
					provider = "Not selected; the first interactive credential save will ask."
				}
				_, err = fmt.Fprintln(cmd.OutOrStdout(), provider)
				return err
			}
			if err := config.ValidateCredentialProvider(args[0]); err != nil {
				return err
			}
			if err := config.PutKeyValueOnDisk("secrets.provider", args[0]); err != nil {
				return err
			}
			if args[0] == "plaintext" {
				config.WarnPlaintextStorage()
			}
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "Default credential provider: %s\n", args[0])
			return err
		},
	}
}
