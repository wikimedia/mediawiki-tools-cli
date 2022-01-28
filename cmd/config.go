package cmd

import (
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/releng/cli/internal/config"
)

func NewConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Display or change configuration settings",
	}
}

func NewConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Shows the raw config",
		Run: func(cmd *cobra.Command, args []string) {
			config.LoadFromDisk().PrettyPrint()
		},
	}
}

func configAttachToCmd(rootCmd *cobra.Command) {
	configCmd := NewConfigCmd()
	configCmd.AddCommand(NewConfigShowCmd())
	rootCmd.AddCommand(configCmd)
}
