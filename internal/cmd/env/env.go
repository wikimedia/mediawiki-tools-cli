package env

import (
	"os"

	"github.com/spf13/cobra"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
)

// Env command for interacting with a .env file in the given directory.
func Env(Short string, directory func() string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "env",
		Short:   Short,
		GroupID: "core",
		RunE:    nil,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			os.Setenv("MWCLI_ENV_COMMAND", "1")
			cobrautil.CallAllPersistentPreRun(cmd, args)
		},
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
