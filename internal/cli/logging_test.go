package cli

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestDiagnosticVerbosity(t *testing.T) {
	previous := slog.Default()
	t.Cleanup(func() { slog.SetDefault(previous) })
	for _, test := range []struct {
		verbosity    int
		debug, trace bool
	}{{0, false, false}, {1, true, false}, {2, true, true}} {
		var buffer bytes.Buffer
		ConfigureLogging(&buffer, test.verbosity)
		slog.Debug("debug-marker", "operation", "fixture")
		slog.Log(context.Background(), TraceLevel, "trace-marker")
		got := buffer.String()
		if strings.Contains(got, "debug-marker") != test.debug || strings.Contains(got, "trace-marker") != test.trace {
			t.Fatalf("verbosity %d: %q", test.verbosity, got)
		}
		if test.debug && !strings.Contains(got, "operation=fixture") {
			t.Fatalf("structured field lost: %q", got)
		}
		if strings.Contains(got, "time=") {
			t.Fatalf("unnecessary timestamp: %q", got)
		}
	}
}
