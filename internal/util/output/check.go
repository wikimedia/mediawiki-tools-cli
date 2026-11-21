package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/fatih/color"
)

// PrintCheck prints a diagnostic result with terminal styling when enabled.
func PrintCheck(out io.Writer, passed bool, format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	if ColorEnabled(out) {
		marker, shade := "⚠️", color.FgYellow
		if passed {
			marker, shade = "✅", color.FgGreen
		}
		label, detail, _ := strings.Cut(message, ": ")
		message = marker + " " + StyledText(out, label, shade, color.Bold)
		if detail != "" {
			message += ": " + detail
		}
	}
	fmt.Fprintln(out, message)
}
