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
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			file := dotenv.FileForDirectory(directory())
			values, err := file.Read()
			if err != nil {
				return err
			}
			for name := range values {
				if err := file.Delete(name); err != nil {
					return err
				}
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "Cleared .env file")
			return err
		},
	}
}
