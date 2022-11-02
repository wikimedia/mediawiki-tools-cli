package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/cmd"
	utilstrings "gitlab.wikimedia.org/repos/releng/cli/internal/util/strings"
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
	Context string
	Level   Level
}

func detectorList() []func(*cobra.Command, string) *Issue {
	return []func(*cobra.Command, string) *Issue{
		// short-required: Short description is always required
		func(theCmd *cobra.Command, cmdStirng string) *Issue {
			if len(theCmd.Short) == 0 {
				return &Issue{
					Command: cmdStirng,
					Level:   ErrorLevel,
					Code:    "short-required",
					Text:    "Short descriptions are required",
				}
			}
			return nil
		},
		// examples-required-when-flagged-lowlevel: Low level commands with flags require at least one example
		func(theCmd *cobra.Command, cmdStirng string) *Issue {
			if len(theCmd.Commands()) == 0 && /*No sub commands*/
				theCmd.Flags().HasFlags() && /*At least one flag*/
				len(utilstrings.SplitMultiline(theCmd.Example)) <= 0 /*No example lines*/ {
				return &Issue{
					Command: cmdStirng,
					Level:   WarningLevel,
					Code:    "examples-required-when-flagged-lowlevel",
					Text:    "Low level commands with flags require at least one example",
				}
			}
			return nil
		},
		// examples-expected: All examples need to start with something that is expected
		// This can be 1) The command name 2) a comment "#" 3) be a blank line (separation)
		// It is common for people to start with `mw`, an alias or for whitespace between lines to be incorrect
		func(theCmd *cobra.Command, cmdStirng string) *Issue {
			for _, line := range utilstrings.SplitMultiline(theCmd.Example) {
				// TODO check all example lines and return an issue for each?
				if len(line) > 0 && strings.Index(line, theCmd.Name()) != 0 && strings.Index(line, "#") != 0 {
					return &Issue{
						Command: cmdStirng,
						Level:   ErrorLevel,
						Code:    "examples-expected-start",
						Text:    "All examples have an expected start",
						Context: ">>> " + line,
					}
				}
			}
			return nil
		},
		// annotations-allowed: Only defined annotations are allowed
		// Generally to help avoid typos
		func(theCmd *cobra.Command, cmdStirng string) *Issue {
			allowedKeys := []string{
				"exitCode",
				"group",
				"mwcli-lint-skip",
				"mwcli-lint-skip-children",
			}
			// TODO check all annotaitons and issue for each
			for key := range theCmd.Annotations {
				if !utilstrings.StringInSlice(key, allowedKeys) {
					return &Issue{
						Command: cmdStirng,
						Level:   ErrorLevel,
						Code:    "annotations-allowed",
						Text:    "Annotation keys must come from allowed list",
						Context: ">>> " + key,
					}
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

	if len(theCmd.Annotations["mwcli-lint-skip"]) == 0 {
		issues = append(issues, detectIssues(theCmd, theCmdName)...)
	}

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
