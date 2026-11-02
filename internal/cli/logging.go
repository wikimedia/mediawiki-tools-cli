package cli

import (
	"io"
	"log/slog"
)

// TraceLevel enables the most detailed diagnostics with -vv.
const TraceLevel = slog.Level(-8)

// ConfigureLogging keeps supplemental diagnostics separate from command output.
func ConfigureLogging(writer io.Writer, verbosity int) {
	level := slog.LevelInfo
	if verbosity == 1 {
		level = slog.LevelDebug
	} else if verbosity >= 2 {
		level = TraceLevel
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(writer, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
			if attr.Key == slog.TimeKey {
				return slog.Attr{}
			}
			if attr.Key == slog.LevelKey && attr.Value.Any() == TraceLevel {
				return slog.String(slog.LevelKey, "TRACE")
			}
			return attr
		},
	})))
}
