package env

import (
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dotenv"
)

func envSet(directory func() string) *cobra.Command {
	return &cobra.Command{
		Use:   "set [name] [value]",
		Short: "Set an environment variable",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return dotenv.FileForDirectory(directory()).Set(args[0], args[1])
		},
	}
}
