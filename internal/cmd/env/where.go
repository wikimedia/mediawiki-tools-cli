package env

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dotenv"
)

func envWhere(directory func() string) *cobra.Command {
	return &cobra.Command{
		Use:   "where",
		Short: "Output the location of the .env file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), dotenv.FileForDirectory(directory()).Path())
			return err
		},
	}
}
