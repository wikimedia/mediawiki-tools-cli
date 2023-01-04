package main

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"gitlab.wikimedia.org/repos/releng/cli/cmd"
	"gitlab.wikimedia.org/repos/releng/cli/tools/lint/detectors"
)

func main() {
	rootCmd := cmd.NewMwCliCmd()

	cobraCommandIssues := detectors.DetectCobraCommandIssuesRoot(rootCmd)
	fileIssues := detectors.DetectFileIssues("./../../")

	issueCount := len(cobraCommandIssues) + len(fileIssues)

	if issueCount == 0 {
		fmt.Println("No issues detected!")
		os.Exit(0)
	}

	fmt.Printf("Found %d issues!\n", issueCount)
	numWarnings := 0
	numErrors := 0

	for _, issue := range cobraCommandIssues {
		if issue.Level == detectors.WarningLevel {
			numWarnings++
			color.Yellow("WARN command %s: (%s) %s", issue.Code, issue.Command, issue.Text)
		}
		if issue.Level == detectors.ErrorLevel {
			numErrors++
			color.Red("ERRR command %s: (%s) %s", issue.Code, issue.Command, issue.Text)
		}
		if issue.Context != "" {
			fmt.Println(issue.Context)
		}
	}

	for _, issue := range fileIssues {
		if issue.Level == detectors.WarningLevel {
			numWarnings++
			color.Yellow("WARN file %s: (%s) %s", issue.Code, issue.File, issue.Text)
		}
		if issue.Level == detectors.ErrorLevel {
			numErrors++
			color.Red("ERRR file %s: (%s) %s", issue.Code, issue.File, issue.Text)
		}
		if issue.Context != "" {
			fmt.Println(issue.Context)
		}
	}

	fmt.Printf("%d warnings and %d errors\n", numWarnings, numErrors)
	if numErrors != 0 {
		os.Exit(1)
	} else {
		fmt.Println("(success) :)")
	}
}
