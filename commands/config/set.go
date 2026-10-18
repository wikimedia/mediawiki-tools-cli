package config

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
	"gitlab.wikimedia.org/repos/releng/cli/internal/secrets"
)

func NewConfigSetCmd() *cobra.Command {
	var stdin, insecureStorage bool
	var provider string
	cmd := &cobra.Command{
		Use:     "set KEY [VALUE]",
		Short:   "Set a configuration value",
		Example: "set telemetry no\nset gerrit.password --stdin",
		Args:    cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if provider != "" {
				if err := config.ValidateCredentialProvider(provider); err != nil {
					return err
				}
			}
			var value string
			if stdin {
				if len(args) != 1 {
					return fmt.Errorf("do not supply VALUE with --stdin")
				}
				var err error
				value, err = secrets.ReadInput(cmd.InOrStdin())
				if err != nil {
					return err
				}
			} else {
				if len(args) != 2 {
					return fmt.Errorf("provide VALUE or --stdin")
				}
				value = args[1]
			}
			key := args[0]
			if provider != "" && !config.SensitiveKey(key) {
				return fmt.Errorf("--provider only applies to credentials")
			}
			if config.SensitiveKey(key) {
				credential, err := config.CredentialForKey(key)
				if err == nil {
					if err := credential.SaveWithProvider(value, insecureStorage, nil, provider); err != nil {
						return err
					}
				} else {
					if !insecureStorage {
						return fmt.Errorf("unknown credential key; use a service login or explicitly select --insecure-storage")
					}
					config.WarnPlaintextStorage()
					if err := config.PutKeyValueOnDisk(key, value); err != nil {
						return err
					}
				}
			} else if err := config.PutKeyValueOnDisk(key, value); err != nil {
				return err
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "Set "+key)
			return err
		},
	}
	cmd.Flags().BoolVar(&stdin, "stdin", false, "Read the value from stdin instead of command arguments")
	cmd.Flags().BoolVar(&insecureStorage, "insecure-storage", false, "Explicitly store a secret in the private plaintext config")
	cmd.Flags().StringVar(&provider, "provider", "", "Override the credential provider (keyring, keepassxc or plaintext)")
	cmd.MarkFlagsMutuallyExclusive("provider", "insecure-storage")
	cmd.RegisterFlagCompletionFunc("provider", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"keyring", "keepassxc", "plaintext"}, cobra.ShellCompDirectiveNoFileComp
	})
	return cmd
}
