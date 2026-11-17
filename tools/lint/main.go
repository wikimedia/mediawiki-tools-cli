package main

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"gitlab.wikimedia.org/repos/releng/cli/cmd"
	"gitlab.wikimedia.org/repos/releng/cli/tools/lint/detectors"
	"gitlab.wikimedia.org/repos/releng/cli/tools/lint/issue"
)

func main() {
	rootCmd := cmd.NewMwCliCmd()

	// Collect all issues
	allIssues := detectors.DetectCobraCommandIssuesRoot(rootCmd)
	// Execute normally adds Cobra's built-in help and completion commands.
	rootCmd.InitDefaultHelpCmd()
	rootCmd.InitDefaultCompletionCmd()
	coverage, hintIssues := detectors.DetectCommandHints(rootCmd)
	allIssues = append(allIssues, hintIssues...)
	allIssues = append(allIssues, detectors.DetectFileIssues("./../../")...)
	allIssues = append(allIssues, detectors.DetectDataIssues()...)

	issueCount := len(allIssues)
	percentage := 0.0
	if coverage.Executable > 0 {
		percentage = 100 * float64(coverage.Classified) / float64(coverage.Executable)
	}
	fmt.Printf("Command hint coverage: %d/%d executable commands (%.1f%%)\n", coverage.Classified, coverage.Executable, percentage)

	if issueCount == 0 {
		fmt.Println("No issues detected!")
		os.Exit(0)
	}

	fmt.Printf("Found %d issues!\n", issueCount)
	numSuggests := 0
	numWarnings := 0
	numErrors := 0

	for _, thisIssue := range allIssues {
		if thisIssue.Level == issue.SuggestLevel {
			numSuggests++
			color.Green("SUGGEST %s: (%s) %s", thisIssue.Target, thisIssue.Code, thisIssue.Text)
		}
		if thisIssue.Context != "" {
			fmt.Println(thisIssue.Context)
		}
	}
	for _, thisIssue := range allIssues {
		if thisIssue.Level == issue.WarningLevel {
			numWarnings++
			color.Yellow("WARN %s: (%s) %s", thisIssue.Target, thisIssue.Code, thisIssue.Text)
		}
		if thisIssue.Context != "" {
			fmt.Println(thisIssue.Context)
		}
	}
	for _, thisIssue := range allIssues {
		if thisIssue.Level == issue.ErrorLevel {
			numErrors++
			color.Red("ERRR %s: (%s) %s", thisIssue.Target, thisIssue.Code, thisIssue.Text)
		}
		if thisIssue.Context != "" {
			fmt.Println(thisIssue.Context)
		}
	}

	fmt.Printf("%d suggestions, %d warnings and %d errors\n", numSuggests, numWarnings, numErrors)
	if numErrors != 0 {
		os.Exit(1)
	} else {
		fmt.Println("(success) :)")
	}
}
