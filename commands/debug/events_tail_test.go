package debug

import (
	"context"
	"io"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
)

func TestDebugEventsTailCommand(t *testing.T) {
	for _, args := range [][]string{{"tail"}, {"tail", "-n", "0"}, {"tail", "--lines", "25"}, {"watch"}} {
		cmd := DebugEventsCmd()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		cmd.SetContext(ctx)
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
	}
	cmd := DebugEventsTailCmd()
	if cmd.Flags().Lookup("lines").DefValue != "10" || cmd.Annotations[cli.TelemetrySkipAnnotation] != "true" {
		t.Fatal("tail must default to 10 events and skip telemetry")
	}
	for _, args := range [][]string{{"-n", "-1"}, {"unexpected"}, {"--lines", "bad"}} {
		cmd := DebugEventsTailCmd()
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Fatalf("accepted %q", args)
		}
	}
}
