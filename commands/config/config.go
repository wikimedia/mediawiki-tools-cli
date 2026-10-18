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
	return cmd
}

func NewSecretsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "secrets", Short: "Manage credential storage and password-manager connections"}
	cmd.AddCommand(NewMigrateSecretsCmd(), NewUseKeePassXCCmd(), NewKeePassXCCmd())
	return cmd
}
