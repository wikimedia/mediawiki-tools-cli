package cmdgloss

import (
	"fmt"
	"io"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// PrintStageHeading separates a stage from preceding command output. Styling is
// detected for the supplied writer, so redirected output remains plain text.
func PrintStageHeading(out io.Writer, name string) {
	heading := lipgloss.NewRenderer(out).NewStyle().Bold(true).Foreground(lipgloss.Color("2"))
	fmt.Fprintf(out, "\n%s\n", heading.Render("==> "+name))
}

// RunStage prints a heading, runs the operation, and reports its duration on
// success. Errors retain their cause and identify the stage that failed.
func RunStage(out io.Writer, name string, run func() error) error {
	PrintStageHeading(out, name)
	start := time.Now()
	if err := run(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	fmt.Fprintf(out, "Completed: %s (%s)\n", name, time.Since(start).Round(time.Millisecond))
	return nil
}
