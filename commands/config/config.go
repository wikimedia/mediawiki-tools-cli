package config

import (
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Display or change configuration settings",
	}
	cmd.AddCommand(NewConfigShowCmd())
	cmd.AddCommand(NewConfigWhereCmd())
	cmd.AddCommand(NewConfigGetCmd())
	cmd.AddCommand(NewConfigSetCmd())
	cmd.AddCommand(NewSecretsCmd())
	cmd.AddCommand(NewProfilesCmd())
	return cmd
}

func NewSecretsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "secrets", Short: "Manage credential storage and password-manager connections"}
	cmd.AddCommand(NewListSecretsCmd(), NewDetachedSecretsCmd(), NewCleanupSecretCmd(), NewSecretStatusCmd(), NewCredentialProviderCmd(), NewMigrateSecretsCmd(), NewTransferSecretCmd(), NewUseKeePassXCCmd(), NewKeePassXCCmd())
	return cmd
}
