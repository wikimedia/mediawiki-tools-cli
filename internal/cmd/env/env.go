package env

import (
	"github.com/spf13/cobra"
)

// Env command for interacting with a .env file in the given directory.
func Env(Short string, directory func() string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: Short,
		RunE:  nil,
	}
	cmd.AddCommand(envDelete(directory))
	cmd.AddCommand(envSet(directory))
	cmd.AddCommand(envGet(directory))
	cmd.AddCommand(envList(directory))
	cmd.AddCommand(envWhere(directory))
	cmd.AddCommand(envClear(directory))
	return cmd
}
