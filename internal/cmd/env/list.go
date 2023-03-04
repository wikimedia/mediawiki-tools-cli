package env

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dotenv"
)

func envList(directory func() string) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all environment variables",
		Run: func(cmd *cobra.Command, args []string) {
			for name, value := range dotenv.FileForDirectory(directory()).List() {
				fmt.Println(name + "=" + value)
			}
		},
	}
}
