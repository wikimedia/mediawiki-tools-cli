package output

import (
	"os"

	"golang.org/x/term"
)

const defaultTerminalWidth = 120

// terminalWidth returns the width of the terminal attached to stdout,
// or defaultTerminalWidth when it cannot be determined (e.g. when piped).
func terminalWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return defaultTerminalWidth
	}
	return w
}

// truncateCell truncates s to maxWidth runes, appending '…' when truncated.
// maxWidth <= 0 means no truncation.
func truncateCell(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxWidth {
		return s
	}
	return string(runes[:maxWidth-1]) + "…"
}
