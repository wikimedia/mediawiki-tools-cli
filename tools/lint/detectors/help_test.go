package detectors

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/tools/lint/issue"
)

func TestLongHelpBudgetIgnoresWrapping(t *testing.T) {
	for _, separator := range []string{" ", "\n"} {
		cmd := &cobra.Command{Long: strings.Repeat("word"+separator, maxLongHelpWords)}
		if got := detectLongHelp(cmd, "mw example"); got != nil {
			t.Fatalf("boundary: %v", got)
		}
		cmd.Long += "extra"
		got := detectLongHelp(cmd, "mw example")
		if got == nil || got.Code != "long-help-size" || got.Level != issue.WarningLevel {
			t.Fatalf("expected guide warning regardless of wrapping: %v", got)
		}
	}
}

func TestExampleHelpBudget(t *testing.T) {
	cmd := &cobra.Command{Example: strings.Repeat("example\n\n", maxExampleLines)}
	if got := detectLongExamples(cmd, "mw example"); got != nil {
		t.Fatalf("blank lines count against budget: %v", got)
	}
	cmd.Example += "example"
	if got := detectLongExamples(cmd, "mw example"); got == nil || got.Code != "example-help-size" {
		t.Fatalf("expected line budget warning: %v", got)
	}
	cmd.Example = strings.Repeat("word ", maxExampleWords+1)
	if got := detectLongExamples(cmd, "mw example"); got == nil {
		t.Fatal("one long line bypassed the word budget")
	}
}
