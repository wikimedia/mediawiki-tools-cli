package cobrautil

import (
	"strings"
)

func IndentExamples(examples string) string {
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
