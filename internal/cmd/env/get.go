package env

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dotenv"
)

func envGet(directory func() string) *cobra.Command {
	return &cobra.Command{
		Use:   "get [name]",
		Short: "Get an environment variable",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			values, err := dotenv.FileForDirectory(directory()).Read()
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), values[args[0]])
			return err
		},
	}
}
