package config

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func NewConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set",
		Short: "Sets a setting on the config",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			keyName := args[0]
			value := args[1]

			k := config.State().OnDiskKoanf
			k.Set(keyName, value)
			err := config.PutDiskConfig(k)
			if err != nil {
				logrus.Error(err)
				os.Exit(1)
			}
			fmt.Println("Set " + keyName + " to " + value)
		},
	}
}
