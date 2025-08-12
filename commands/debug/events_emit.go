package debug

import (
	"os"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

func DebugEventsEmitCmd() *cobra.Command {
	return &cobra.Command{
		Hidden:  debugCommandsAreHidden(),
		Use:     "submit",
		Aliases: []string{"emit"},
		Short:   "Submit events now",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Println("Submitting events")
			cli.NewEvents(cli.UserDirectoryPath() + string(os.PathSeparator) + ".events").EmitEvents()
		},
	}
}
