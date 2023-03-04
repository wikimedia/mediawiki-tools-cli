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
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(dotenv.FileForDirectory(directory()).Path())
		},
	}
}
