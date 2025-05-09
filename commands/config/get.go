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
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			keyName := args[0]

			k := config.State().OnDiskKoanf
			v := k.Get(keyName)

			fmt.Println(v)
		},
	}
}
