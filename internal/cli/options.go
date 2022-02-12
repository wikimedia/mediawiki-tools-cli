package cli

type Options struct {
	// NoInteraction means commands should not ask for user interaction
	NoInteraction bool
}

// Options that are global throughout the CLI
var Opts Options
