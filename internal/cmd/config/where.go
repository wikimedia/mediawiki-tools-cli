package config

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/config"
)

func NewConfigWhereCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "where",
		Short: "Outputs the path to the config file",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(config.Path())
		},
	}
}
