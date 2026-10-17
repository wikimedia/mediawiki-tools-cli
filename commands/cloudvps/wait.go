package cloudvps

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Poll immediately, honor cancellation, and stop on terminal OpenStack errors.
func waitForResource(ctx context.Context, target string, status func(context.Context) (string, error)) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		current, err := status(ctx)
		if err != nil {
			return err
		}
		if current == target {
			return nil
		}
		if strings.HasPrefix(strings.ToLower(current), "error") {
			return fmt.Errorf("resource entered %s state", current)
		}
		timer := time.NewTimer(2 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
