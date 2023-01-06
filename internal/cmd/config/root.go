package config

import (
	"github.com/spf13/cobra"
)

func NewConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Display or change configuration settings",
	}
	cmd.AddCommand(NewConfigShowCmd())
	cmd.AddCommand(NewConfigWhereCmd())
	cmd.AddCommand(NewConfigGetCmd())
	cmd.AddCommand(NewConfigSetCmd())
	return cmd
}
