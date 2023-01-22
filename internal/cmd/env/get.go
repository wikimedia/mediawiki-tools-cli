package env

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/dotenv"
)

func envGet(directory func() string) *cobra.Command {
	return &cobra.Command{
		Use:   "get [name]",
		Short: "Get an environment variable",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(dotenv.FileForDirectory(directory()).Get(args[0]))
		},
	}
}
