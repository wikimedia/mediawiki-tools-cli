package config

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func NewConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "Gets a settings from the config",
		Run: func(cmd *cobra.Command, args []string) {
			// TODO get dynamically...
			if args[0] == "dev_mode" {
				fmt.Println(config.LoadFromDisk().DevMode)
			}
			if args[0] == "telemetry" {
				fmt.Println(config.LoadFromDisk().Telemetry)
			}
		},
	}
}
