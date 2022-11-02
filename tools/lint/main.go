package main

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/cmd"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/strings"
)

type Level uint32

const (
	ErrorLevel Level = iota
	WarningLevel
)

type Issue struct {
	Command string
	Code    string
	Text    string
	Level   Level
}

func detectorList() []func(*cobra.Command, string) *Issue {
	return []func(*cobra.Command, string) *Issue{
		// examples-required-flagged-lowlevel: Low level commands with flags require at least one example
		func(theCmd *cobra.Command, cmdStirng string) *Issue {
			if len(theCmd.Commands()) == 0 && /*No sub commands*/
				theCmd.Flags().HasFlags() && /*At least one flag*/
				len(strings.SplitMultiline(theCmd.Example)) <= 0 /*No example lines*/ {
				return &Issue{
					Command: cmdStirng,
					Level:   WarningLevel,
					Code:    "examples-required-lowlevel",
					Text:    "Low level commands with flags require at least one example",
				}
			}
			return nil
		},
	}
}

func detectIssues(theCmd *cobra.Command, cmdString string) []Issue {
	issues := []Issue{}
	for _, detector := range detectorList() {
		issue := detector(theCmd, cmdString)
		if issue != nil {
			issues = append(issues, *issue)
		}
	}
	return issues
}

func detectIssuesRecursively(theCmd *cobra.Command, parentNamePrefix string) []Issue {
	issues := []Issue{}
	theCmdName := parentNamePrefix + theCmd.Name()
	issues = append(issues, detectIssues(theCmd, theCmdName)...)

	if len(theCmd.Annotations["mwcli-lint-skip-children"]) == 0 {
		for _, nextCmd := range theCmd.Commands() {
			issues = append(issues, detectIssuesRecursively(nextCmd, theCmdName+" ")...)
		}
	}

	return issues
}

func detectIssuesRoot(rootCmd *cobra.Command) []Issue {
	return detectIssuesRecursively(rootCmd, "")
}

func main() {
	rootCmd := cmd.NewMwCliCmd()

	issues := detectIssuesRoot(rootCmd)

	if len(issues) == 0 {
		fmt.Println("No issues detected!")
		os.Exit(0)
	}

	fmt.Printf("Found %d issues!\n", len(issues))
	numWarnings := 0
	numErrors := 0
	for _, issue := range issues {
		if issue.Level == WarningLevel {
			numWarnings++
			color.Yellow("WARN %s: (%s) %s", issue.Code, issue.Command, issue.Text)
		}
		if issue.Level == ErrorLevel {
			numErrors++
			color.Red("ERRR %s: (%s) %s", issue.Code, issue.Command, issue.Text)
		}
	}
	fmt.Printf("%d warnings and %d errors\n", numWarnings, numErrors)
	if numErrors != 0 {
		os.Exit(1)
	} else {
		fmt.Println("(success) :)")
	}
}
