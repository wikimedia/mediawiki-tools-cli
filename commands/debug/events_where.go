package debug

import (
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

func DebugEventsWhereCmd() *cobra.Command {
	return &cobra.Command{
		Hidden: debugCommandsAreHidden(),
		Use:    "where",
		Short:  "Show telemetry event queue paths",
		Run: func(cmd *cobra.Command, args []string) {
			events := cli.NewEvents(cli.EventFilePath())
			cmd.Printf("Primary event file: %s\n", events.EventPaths()[0])
			cmd.Printf("Temporary fallback event file: %s\n", events.FallbackEventFile())
			cmd.Printf("Temporary queue state directory: %s\n", events.FallbackDir)
		},
	}
}
