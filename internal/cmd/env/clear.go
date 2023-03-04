package env

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dotenv"
)

func envClear(directory func() string) *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Clears all values from the .env file",
		Run: func(cmd *cobra.Command, args []string) {
			file := dotenv.FileForDirectory(directory())
			for name := range file.List() {
				file.Delete(name)
			}
			fmt.Println("Cleared .env file")
		},
	}
}
