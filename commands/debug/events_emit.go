package debug

import (
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/eventlogging"
)

func DebugEventsEmitCmd() *cobra.Command {
	return &cobra.Command{
		Hidden:  debugCommandsAreHidden(),
		Use:     "submit",
		Aliases: []string{"emit"},
		Short:   "Submit events now",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Println("Submitting events")
			eventlogging.EmitEvents()
		},
	}
}
