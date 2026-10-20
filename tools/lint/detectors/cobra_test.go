package detectors

import (
	"testing"

	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/tools/lint/issue"
)

func TestLongHelpRequiredForExecutableCommandsWithInputs(t *testing.T) {
	cmd := &cobra.Command{Use: "run", Run: func(*cobra.Command, []string) {}}
	cmd.Flags().String("file", "", "Input file")
	if got := detectLongHelpRequired(cmd, "mw run"); got == nil || got.Code != "long-help-required" || got.Level != issue.SuggestLevel {
		t.Fatalf("expected long-help suggestion for command with flags, got %v", got)
	}

	cmd.Long = "Explains the command."
	if got := detectLongHelpRequired(cmd, "mw run"); got != nil {
		t.Fatalf("command with long help should not be flagged: %v", got)
	}

	cmd = &cobra.Command{Use: "version", Run: func(*cobra.Command, []string) {}}
	if got := detectLongHelpRequired(cmd, "mw version"); got != nil {
		t.Fatalf("simple command should not require long help: %v", got)
	}

	root := &cobra.Command{Use: "mw"}
	root.PersistentFlags().Bool("json", false, "JSON output")
	child := &cobra.Command{Use: "status", Run: func(*cobra.Command, []string) {}}
	root.AddCommand(child)
	if got := detectLongHelpRequired(child, "mw status"); got != nil {
		t.Fatalf("inherited root flags should not require command-specific long help: %v", got)
	}
}

func TestPositionalArgsRequireValidation(t *testing.T) {
	cmd := &cobra.Command{Use: "get <page>", Run: func(*cobra.Command, []string) {}}
	if got := detectPositionalArgsValidated(cmd, "mw wiki get"); got == nil || got.Code != "positional-args-unvalidated" || got.Level != issue.WarningLevel {
		t.Fatalf("expected validation warning, got %v", got)
	}

	cmd.Args = cobra.ExactArgs(1)
	if got := detectPositionalArgsValidated(cmd, "mw wiki get"); got != nil {
		t.Fatalf("validated command should not be flagged: %v", got)
	}

	cmd.Args = nil
	cmd.DisableFlagParsing = true
	if got := detectPositionalArgsValidated(cmd, "mw wiki get"); got != nil {
		t.Fatalf("passthrough command should not be flagged: %v", got)
	}
}

func TestUseHasPositionalArgs(t *testing.T) {
	for _, use := range []string{"get <page>", "get page", "run [flags] [command...]", "cp source... destination"} {
		if !useHasPositionalArgs(use) {
			t.Errorf("expected positional arguments in %q", use)
		}
	}
	for _, use := range []string{"help", "list [flags]", "run [--json]"} {
		if useHasPositionalArgs(use) {
			t.Errorf("did not expect positional arguments in %q", use)
		}
	}
}

func TestSiblingCommandNameCollisions(t *testing.T) {
	parent := &cobra.Command{Use: "wiki"}
	parent.AddCommand(
		&cobra.Command{Use: "get", Aliases: []string{"show"}},
		&cobra.Command{Use: "show"},
		&cobra.Command{Use: "list"},
	)
	issues := detectSiblingCommandNameCollisions(parent, "mw wiki")
	if len(issues) != 1 || issues[0].Code != "command-name-collision" || issues[0].Level != issue.ErrorLevel {
		t.Fatalf("expected one sibling name collision, got %#v", issues)
	}

	parent.Commands()[0].Aliases = []string{"fetch"}
	if issues := detectSiblingCommandNameCollisions(parent, "mw wiki"); len(issues) != 0 {
		t.Fatalf("unique sibling names should not be flagged: %#v", issues)
	}
}

func TestFlagNamesUseLowercaseHyphenatedStyle(t *testing.T) {
	cmd := &cobra.Command{Use: "run"}
	cmd.Flags().Bool("dry_run", false, "Preview without applying changes")
	cmd.Flags().Bool("dry--run", false, "Preview without applying changes")
	if got := detectFlagNames(cmd, "mw run"); got == nil || got.Code != "flag-name-style" || got.Level != issue.SuggestLevel {
		t.Fatalf("expected a flag naming suggestion, got %v", got)
	}

	cmd = &cobra.Command{Use: "run"}
	cmd.Flags().Bool("dry-run", false, "Preview without applying changes")
	cmd.Flags().Bool("db2-name", false, "Use the second database name")
	if got := detectFlagNames(cmd, "mw run"); got != nil {
		t.Fatalf("lowercase hyphenated flag should pass, got %v", got)
	}
}

func TestFlagsRequireHelpText(t *testing.T) {
	cmd := &cobra.Command{Use: "run"}
	cmd.Flags().Bool("dry-run", false, "")
	if got := detectFlagHelp(cmd, "mw run"); got == nil || got.Code != "flag-help-required" || got.Level != issue.WarningLevel {
		t.Fatalf("expected missing flag help warning, got %v", got)
	}

	cmd = &cobra.Command{Use: "run"}
	cmd.Flags().Bool("dry-run", false, "Preview without applying changes")
	if got := detectFlagHelp(cmd, "mw run"); got != nil {
		t.Fatalf("documented flag should pass, got %v", got)
	}
}
