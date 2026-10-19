package detectors

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/tools/lint/issue"
)

const (
	maxLongHelpWords = 450
	maxExampleWords  = 150
	maxExampleLines  = 12
)

func detectLongHelp(cmd *cobra.Command, path string) *issue.Issue {
	words := len(strings.Fields(cmd.Long))
	if words <= maxLongHelpWords {
		return nil
	}
	return &issue.Issue{
		Target: "cmd: " + path, Level: issue.WarningLevel, Code: "long-help-size",
		Text:    "Move detailed workflows into a guide and link to it from help",
		Context: fmt.Sprintf("Long has %d words; suggested maximum is %d", words, maxLongHelpWords),
	}
}

func detectLongExamples(cmd *cobra.Command, path string) *issue.Issue {
	lines := 0
	for _, line := range strings.Split(cmd.Example, "\n") {
		if strings.TrimSpace(line) != "" {
			lines++
		}
	}
	words := len(strings.Fields(cmd.Example))
	if lines <= maxExampleLines && words <= maxExampleWords {
		return nil
	}
	return &issue.Issue{
		Target: "cmd: " + path, Level: issue.WarningLevel, Code: "example-help-size",
		Text:    "Keep a few short examples in help; put complete walkthroughs in a guide",
		Context: fmt.Sprintf("Example has %d nonblank lines and %d words; suggested maxima are %d lines and %d words", lines, words, maxExampleLines, maxExampleWords),
	}
}
