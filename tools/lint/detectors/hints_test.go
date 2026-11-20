package detectors

import (
	"testing"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/commandmeta"
	"gitlab.wikimedia.org/repos/releng/cli/tools/lint/issue"
)

func TestHintCoverageIncludesAllExecutableCommands(t *testing.T) {
	run := func(*cobra.Command, []string) {}
	hints := commandmeta.Hints{DataAccess: []commandmeta.DataAccess{commandmeta.DataUnknown}}
	root := commandmeta.Set(&cobra.Command{Use: "mw"}, hints)
	classified := commandmeta.Set(&cobra.Command{Use: "classified", Aliases: []string{"alias"}, Run: run}, hints)
	hidden := &cobra.Command{Use: "hidden", Hidden: true, Run: run}
	imported := &cobra.Command{Use: "imported", Run: run, Annotations: map[string]string{
		"mwcli-lint-skip": "upstream", "mwcli-lint-skip-children": "upstream",
	}}
	imported.AddCommand(&cobra.Command{Use: "child", RunE: func(*cobra.Command, []string) error { return nil }})
	runnableParent := commandmeta.Set(&cobra.Command{Use: "parent", Run: run}, hints)
	runnableParent.AddCommand(&cobra.Command{Use: "group"})
	root.AddCommand(classified, hidden, imported, runnableParent)
	coverage, issues := DetectCommandHints(root)
	if coverage.Classified != 2 || coverage.Executable != 5 || len(issues) != 3 {
		t.Fatalf("coverage = %+v, issues = %+v", coverage, issues)
	}
	for _, got := range issues {
		if got.Level != issue.WarningLevel || got.Code != "command-hints-missing" {
			t.Fatalf("missing hints must warn: %+v", got)
		}
	}
}

func TestInvalidHintsAreNotCoverage(t *testing.T) {
	cmd := &cobra.Command{Use: "run", Run: func(*cobra.Command, []string) {}, Annotations: map[string]string{commandmeta.ReadOnlyAnnotation: "true"}}
	coverage, issues := DetectCommandHints(cmd)
	if coverage.Classified != 0 || coverage.Executable != 1 || len(issues) != 1 || issues[0].Level != issue.ErrorLevel || issues[0].Code != "command-hints-invalid" {
		t.Fatalf("partial hints must fail validation: %+v / %+v", coverage, issues)
	}
	if found := detectAllowedAnnotations(cmd, "run"); found != nil {
		t.Fatalf("hint annotation rejected: %+v", found)
	}
	coverage, issues = DetectCommandHints(&cobra.Command{Use: "group"})
	if coverage.Executable != 0 || len(issues) != 0 {
		t.Fatalf("non-executable groups must not need hints: %+v / %+v", coverage, issues)
	}
}
