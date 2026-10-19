package detectors

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/tools/lint/issue"
)

const (
	longHelpReviewWords = 450
	exampleReviewWords  = 150
	exampleReviewLines  = 12
)

func detectLongHelp(cmd *cobra.Command, path string) *issue.Issue {
	words := len(strings.Fields(cmd.Long))
	if words <= longHelpReviewWords {
		return nil
	}
	return &issue.Issue{
		Target: "cmd: " + path, Level: issue.SuggestLevel, Code: "long-help-size",
		Text:    "Review long help for readability; keep command behavior here and move broader workflows to guides",
		Context: fmt.Sprintf("Long has %d words; review threshold is %d (not a limit)", words, longHelpReviewWords),
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
	if lines <= exampleReviewLines && words <= exampleReviewWords {
		return nil
	}
	return &issue.Issue{
		Target: "cmd: " + path, Level: issue.SuggestLevel, Code: "example-help-size",
		Text:    "Review examples for clarity and repetition; keep examples needed to use this command",
		Context: fmt.Sprintf("Example has %d nonblank lines and %d words; review thresholds are %d lines and %d words (not limits)", lines, words, exampleReviewLines, exampleReviewWords),
	}
}
