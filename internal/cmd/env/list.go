package env

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/pkg/dotenv"
)

func envList(directory func() string) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all environment variables",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			values, err := dotenv.FileForDirectory(directory()).Read()
			if err != nil {
				return err
			}
			names := make([]string, 0, len(values))
			for name := range values {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), name+"="+values[name]); err != nil {
					return err
				}
			}
			return nil
		},
	}
}
