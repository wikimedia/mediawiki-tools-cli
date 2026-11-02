package env

import (
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dotenv"
)

func envHas(directory func() string) *cobra.Command {
	return &cobra.Command{
		Use:   "has [name]",
		Short: "Exits 0 if the env var exists, exits 1 if it does not",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			values, err := dotenv.FileForDirectory(directory()).Read()
			if err != nil {
				return err
			}
			message := "Env var exists"
			if _, exists := values[args[0]]; !exists {
				message = "Env var does not exist"
				if cmd.Root().Annotations == nil {
					cmd.Root().Annotations = map[string]string{}
				}
				cmd.Root().Annotations["exitCode"] = "1"
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), message)
			return err
		},
	}
}
