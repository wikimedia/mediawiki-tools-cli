package detectors

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/tools/lint/issue"
)

func TestLongHelpReadabilityIgnoresWrapping(t *testing.T) {
	for _, separator := range []string{" ", "\n"} {
		cmd := &cobra.Command{Long: strings.Repeat("word"+separator, longHelpReviewWords)}
		if got := detectLongHelp(cmd, "mw example"); got != nil {
			t.Fatalf("boundary: %v", got)
		}
		cmd.Long += "extra"
		got := detectLongHelp(cmd, "mw example")
		if got == nil || got.Code != "long-help-size" || got.Level != issue.SuggestLevel {
			t.Fatalf("expected readability suggestion regardless of wrapping: %v", got)
		}
	}
}

func TestExampleHelpReadability(t *testing.T) {
	cmd := &cobra.Command{Example: strings.Repeat("example\n\n", exampleReviewLines)}
	if got := detectLongExamples(cmd, "mw example"); got != nil {
		t.Fatalf("blank lines count against the review threshold: %v", got)
	}
	cmd.Example += "example"
	if got := detectLongExamples(cmd, "mw example"); got == nil || got.Code != "example-help-size" || got.Level != issue.SuggestLevel {
		t.Fatalf("expected line readability suggestion: %v", got)
	}
	cmd.Example = strings.Repeat("word ", exampleReviewWords+1)
	if got := detectLongExamples(cmd, "mw example"); got == nil || got.Level != issue.SuggestLevel {
		t.Fatal("expected a word-count readability suggestion")
	}
}
