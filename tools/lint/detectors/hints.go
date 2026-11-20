package detectors

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	"gitlab.wikimedia.org/repos/releng/cli/tools/lint/issue"
)

type HintCoverage struct {
	Classified int
	Executable int
}

// DetectCommandHints includes hidden and imported commands, ignoring lint skips.
func DetectCommandHints(root *cobra.Command) (HintCoverage, []issue.Issue) {
	var coverage HintCoverage
	var issues []issue.Issue
	var visit func(*cobra.Command)
	visit = func(cmd *cobra.Command) {
		if cmd.Runnable() {
			coverage.Executable++
			_, err := commandmeta.Read(cmd)
			switch {
			case err == nil:
				coverage.Classified++
			case errors.Is(err, commandmeta.ErrMissingHints):
				issues = append(issues, issue.Issue{
					Target: "cmd: " + cmd.CommandPath(), Code: "command-hints-missing",
					Level: issue.WarningLevel, Text: "Executable command has no capability hints",
				})
			default:
				issues = append(issues, issue.Issue{
					Target: "cmd: " + cmd.CommandPath(), Code: "command-hints-invalid",
					Level: issue.ErrorLevel, Text: fmt.Sprintf("Invalid capability hints: %v", err),
				})
			}
		}
		for _, child := range cmd.Commands() {
			visit(child)
		}
	}
	visit(root)
	return coverage, issues
}
