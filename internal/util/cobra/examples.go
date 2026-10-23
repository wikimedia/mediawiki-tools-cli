package cobrautil

import (
	"strings"

	"github.com/spf13/cobra"
)

func NormalizeExample(examples string) string {
	lines := strings.Split(examples, "\n")
	var indentedLines []string
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine != "" {
			indentedLines = append(indentedLines, "  "+trimmedLine)
		}
	}
	return strings.Join(indentedLines, "\n")
}

// CommandExamples renders copyable examples without changing their source definitions.
func CommandExamples(cmd *cobra.Command) string {
	lines := strings.Split(cmd.Example, "\n")
	for i, line := range lines {
		text := strings.TrimSpace(line)
		switch {
		case text == cmd.Name() || strings.HasPrefix(text, cmd.Name()+" "):
			text = cmd.CommandPath() + strings.TrimPrefix(text, cmd.Name())
		case strings.HasPrefix(text, "mw "):
			text = cmd.Root().Name() + strings.TrimPrefix(text, "mw")
		case strings.HasPrefix(text, "wm "):
			text = cmd.Root().Name() + strings.TrimPrefix(text, "wm")
		}
		if text != "" {
			lines[i] = "  " + text
		}
	}
	return strings.Join(lines, "\n")
}
