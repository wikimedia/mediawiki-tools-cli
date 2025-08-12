package debug

import (
	"os"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

func DebugEventsCatCmd() *cobra.Command {
	return &cobra.Command{
		Hidden:  debugCommandsAreHidden(),
		Use:     "cat",
		Aliases: []string{"list"},
		Short:   "List events pending submission",
		Run: func(cmd *cobra.Command, args []string) {
			for _, line := range cli.NewEvents(cli.UserDirectoryPath() + string(os.PathSeparator) + ".events").RawEvents() {
				cmd.Println(line)
			}
		},
	}
}
