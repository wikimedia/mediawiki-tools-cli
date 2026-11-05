package enterprise

import (
	"context"
	"errors"
	"fmt"
	"os"
	ossignal "os/signal"
	"time"

	"github.com/spf13/cobra"
	api "gitlab.wikimedia.org/repos/releng/cli/internal/enterprise"
)

func newStreamCmd(profile *string) *cobra.Command {
	parent := &cobra.Command{Use: "stream", Short: "Follow realtime Enterprise changes (paid access)"}
	for _, resource := range []string{"articles", "wikidata"} {
		var opts queryOptions
		var since, checkpoint string
		var parts []int
		var limit int
		var duration time.Duration
		cmd := &cobra.Command{
			Use: resource, Short: "Stream " + resource + " changes as NDJSON",
			Long:    "Follow realtime Enterprise changes as NDJSON until interrupted. Reconnects use per-partition publication timestamps and may replay events; consumers should deduplicate by event identifier. --checkpoint persists recovery state for this exact query. History is subject to server retention (currently 48 hours). Wikidata is beta. With --fields, event.partition and event.date_published are also requested for recovery.",
			Example: resource + " --wiki enwiki --limit 100\n" + resource + " --checkpoint enterprise-checkpoint.json", Args: cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				if limit < 0 || duration < 0 {
					return fmt.Errorf("--limit and --duration must be zero or greater")
				}
				var start time.Time
				if since != "" {
					var err error
					start, err = time.Parse(time.RFC3339Nano, since)
					if err != nil {
						return fmt.Errorf("--since must be an RFC3339 timestamp")
					}
				}
				query, err := opts.body(false)
				if err != nil {
					return err
				}
				c, err := client(*profile)
				if err != nil {
					return err
				}
				ctx, stop := ossignal.NotifyContext(cmd.Context(), os.Interrupt)
				defer stop()
				if duration > 0 {
					var cancel context.CancelFunc
					ctx, cancel = context.WithTimeout(ctx, duration)
					defer cancel()
				}
				err = c.Stream(ctx, api.StreamOptions{Resource: resource, Query: query, Parts: parts, Since: start, Limit: limit, Checkpoint: checkpoint}, cmd.OutOrStdout(), cmd.ErrOrStderr())
				if errors.Is(err, context.Canceled) || (duration > 0 && errors.Is(err, context.DeadlineExceeded) && cmd.Context().Err() == nil) {
					return nil
				}
				return err
			},
		}
		opts.flags(cmd, false)
		cmd.Flags().StringVar(&since, "since", "", "Start at an RFC3339 timestamp within stream retention")
		cmd.Flags().StringVar(&checkpoint, "checkpoint", "", "Persist and resume per-partition stream state in this file")
		cmd.Flags().IntSliceVar(&parts, "parts", nil, "Consume these comma-separated partitions (0-9; default all)")
		cmd.Flags().IntVar(&limit, "limit", 0, "Stop after this many events (0 means unlimited)")
		cmd.Flags().DurationVar(&duration, "duration", 0, "Stop after this duration (0 means unlimited)")
		parent.AddCommand(cmd)
	}
	return parent
}
