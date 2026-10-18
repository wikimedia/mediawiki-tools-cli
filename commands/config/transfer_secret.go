package config

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func NewTransferSecretCmd() *cobra.Command {
	var provider string
	cmd := &cobra.Command{
		Use:     "transfer KEY",
		Short:   "Copy a linked credential to another provider",
		Long:    "Copy a saved credential to another provider, verify it, then switch its configuration reference. External source entries are retained; plaintext configuration is removed after verified secure storage. The configured default is unchanged.",
		Example: "transfer gerrit.password --provider keyring",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			credential, err := config.CredentialForKey(args[0])
			if err != nil {
				return err
			}
			if err := credential.TransferTo(provider); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Transferred %s to %s.\n", args[0], provider)
			return err
		},
	}
	cmd.Flags().StringVar(&provider, "provider", "", "Destination provider (keyring, keepassxc or plaintext)")
	_ = cmd.MarkFlagRequired("provider")
	cmd.RegisterFlagCompletionFunc("provider", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"keyring", "keepassxc", "plaintext"}, cobra.ShellCompDirectiveNoFileComp
	})
	return cmd
}
