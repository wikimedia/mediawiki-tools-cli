package debug

import (
	"context"
	"errors"
	"fmt"
	"os"
	ossignal "os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
)

func DebugEventsTailCmd() *cobra.Command {
	var lines int
	cmd := &cobra.Command{
		Use:     "tail",
		Aliases: []string{"watch"},
		Short:   "Follow locally queued telemetry events",
		Hidden:  debugCommandsAreHidden(),
		Long: `Print the last 10 pending telemetry events, then follow new events like
tail -f. Each event is printed as one JSON line, without clearing the screen,
so output can be piped to other commands. Use --lines 0 to show only new events.

Follows the primary and temporary fallback queues, including pending upload
batches, across rotation. Existing events use the same queue order as events cat,
not a global timestamp sort. Queues are checked every 200ms; events created and
successfully submitted between checks may not be observed. This is a local
queue observer, not a complete audit log or a subscription to the intake server.

This command does not enable telemetry, generate events or submit them.
Read errors stop the command. Following is enabled by default; press Ctrl+C to stop.`,
		Example: cobrautil.NormalizeExample("tail\ntail --lines 0\ntail -n 25"),
		Args:    cobra.NoArgs,
		Annotations: map[string]string{
			cli.TelemetrySkipAnnotation: "true",
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if lines < 0 {
				return fmt.Errorf("--lines must be zero or greater")
			}
			ctx, stop := ossignal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			err := cli.NewEvents(cli.EventFilePath()).TailEvents(ctx, cmd.OutOrStdout(), lines)
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		},
	}
	cmd.Flags().IntVarP(&lines, "lines", "n", 10, "Number of pending events to print before following new events")
	return cmd
}
