package debug

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

func DebugEventsPingCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "ping",
		Short:  "Check whether the event intake endpoint responds without sending an event",
		Hidden: true,
		Annotations: map[string]string{
			cli.TelemetrySkipAnnotation: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			req, err := http.NewRequestWithContext(ctx, http.MethodHead, cli.EventIntakeEndpoint, nil)
			if err != nil {
				return err
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return fmt.Errorf("could not reach event intake endpoint: %w", err)
			}
			defer resp.Body.Close()

			cmd.Printf("Event intake responded with HTTP %s. No event was sent.\n", resp.Status)
			if resp.StatusCode >= http.StatusInternalServerError {
				return fmt.Errorf("event intake returned a server error")
			}
			cmd.Println("This checks HTTPS reachability only; it does not test event submission.")
			return nil
		},
	}
}
