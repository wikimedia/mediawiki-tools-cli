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
	return cmd
}
