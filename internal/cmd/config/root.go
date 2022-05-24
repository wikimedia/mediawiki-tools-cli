package config

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func NewConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Display or change configuration settings",
	}
	cmd.AddCommand(NewConfigShowCmd())
	cmd.AddCommand(NewConfigWhereCmd())
	return cmd
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

func NewConfigWhereCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "where",
		Short: "Outputs the path to the config file",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(config.Path())
		},
	}
}
