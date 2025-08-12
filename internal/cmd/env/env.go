package env

import (
	"github.com/spf13/cobra"
)

// Env command for interacting with a .env file in the given directory.
// This command can be used in multiple different settings, simply by passing in a different directory
func Env(Short string, directory func() string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "env",
		Short:   Short,
		GroupID: "core",
		RunE:    nil,
	}
	cmd.AddCommand(envDelete(directory))
	cmd.AddCommand(envSet(directory))
	cmd.AddCommand(envGet(directory))
	cmd.AddCommand(envList(directory))
	cmd.AddCommand(envWhere(directory))
	cmd.AddCommand(envClear(directory))
	cmd.AddCommand(envHas(directory))
	return cmd
}
