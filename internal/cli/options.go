package cli

import (
	"os"

	"golang.org/x/term"
)

type Options struct {
	// NoInteraction means commands should not ask for user interaction
	NoInteraction bool
	// Verbosity enables debug diagnostics at 1 and trace diagnostics at 2.
	Verbosity int
	// Telemetry do we want to record telemetry data?
	Telemetry bool
}

// Options that are global throughout the CLI.
var Opts Options

// CanAskForInput reports whether commands may safely ask interactive questions.
func CanAskForInput() bool {
	return !Opts.NoInteraction && term.IsTerminal(int(os.Stdin.Fd()))
}
