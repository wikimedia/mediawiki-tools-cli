package output

import (
	"os"

	"github.com/mattn/go-isatty"
)

func shouldColor() bool {
	// From https://github.com/cli/cli/blob/bf83c660a1ae486d582117e0a174f8e109b64775/pkg/iostreams/iostreams.go#L389
	// Note: This currently only checks os.Stdout, and not the actul writer in use...
	stdoutIsTTY := isTerminal(os.Stdout)
	return stdoutIsTTY
}

func isTerminal(f *os.File) bool {
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}
