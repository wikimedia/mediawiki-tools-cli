package env

import (
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dotenv"
)

func envDelete(directory func() string) *cobra.Command {
	return &cobra.Command{
		Use:   "delete [name]",
		Short: "Deletes an environment variable",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return dotenv.FileForDirectory(directory()).Delete(args[0])
		},
	}
}
