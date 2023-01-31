package env

import (
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/dotenv"
)

func envDelete(directory func() string) *cobra.Command {
	return &cobra.Command{
		Use:   "delete [name]",
		Short: "Deletes an environment variable",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			dotenv.FileForDirectory(directory()).Delete(args[0])
		},
	}
}
