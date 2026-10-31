package config

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func NewTransferSecretCmd() *cobra.Command {
	var provider string
	var keepSource bool
	cmd := &cobra.Command{
		Use:     "transfer KEY",
		Short:   "Move a linked credential to another provider",
		Long:    "Verify and save the destination, switch the reference, then remove the source copy if it is no longer linked. KeePassXC requests GUI deletion approval. --keep-source retains the source and tracks it for later cleanup. A cleanup failure leaves the verified destination active and keeps the source record. The configured default is unchanged.",
		Example: "transfer gerrit.profiles.addshore.password --provider keyring",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			credential, err := config.CredentialForKey(args[0])
			if err != nil {
				return err
			}
			source, err := credential.CredentialProvider("")
			if err != nil {
				return err
			}
			if err := credential.TransferTo(provider, config.TransferOptions{KeepSource: keepSource}); err != nil {
				return err
			}
			message := "Source cleanup completed; entries still linked elsewhere were retained."
			if keepSource {
				message = "Source retained; use config secrets detached to review it."
				if source == "plaintext" {
					message = "Plaintext source retained in configuration; use config secrets list to review it."
				}
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Transferred %s to %s.\n%s\n", args[0], provider, message)
			return err
		},
	}
	cmd.Flags().BoolVar(&keepSource, "keep-source", false, "Keep and track the original provider entry instead of deleting it after transfer")
	cmd.Flags().StringVar(&provider, "provider", "", "Destination provider (keyring, windows-credential-manager, keepassxc or plaintext)")
	_ = cmd.MarkFlagRequired("provider")
	cmd.RegisterFlagCompletionFunc("provider", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return []string{"keyring", "windows-credential-manager", "keepassxc", "plaintext"}, cobra.ShellCompDirectiveNoFileComp
	})
	return cmd
}
