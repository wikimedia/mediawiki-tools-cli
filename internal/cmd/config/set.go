package config

import (
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func NewConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set",
		Short: "Sets a setting on the config",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			// TODO do this dynamically...
			// TODO require 2 args...
			c := config.LoadFromDisk()
			if args[0] == "dev_mode" {
				c.DevMode = args[1]
			}
			if args[0] == "telemetry" {
				c.Telemetry = args[1]
			}
			c.WriteToDisk()
		},
	}
}
