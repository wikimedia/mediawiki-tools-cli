package env

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dotenv"
)

func envHas(directory func() string) *cobra.Command {
	return &cobra.Command{
		Use:   "has [name]",
		Short: "Exits 0 if the env var exists, exits 1 if it does not",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if dotenv.FileForDirectory(directory()).Has(args[0]) {
				fmt.Println("Env var exists")
				os.Exit(0)
			} else {
				fmt.Println("Env var does not exist")
				os.Exit(1)
			}
		},
	}
}
