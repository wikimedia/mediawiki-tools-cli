package detectors

import (
	"strings"

	"github.com/spf13/cobra"
	utilstrings "gitlab.wikimedia.org/repos/releng/cli/internal/util/strings"
	"gitlab.wikimedia.org/repos/releng/cli/tools/lint/issue"
)

func cobraCommandDetectorList() []func(*cobra.Command, string) *issue.Issue {
	return []func(*cobra.Command, string) *issue.Issue{
		// short-required: Short description is always required
		func(theCmd *cobra.Command, cmdStirng string) *issue.Issue {
			if len(theCmd.Short) == 0 {
				return &issue.Issue{
					Target: "cmd: " + cmdStirng,
					Level:  issue.ErrorLevel,
					Code:   "short-required",
					Text:   "Short descriptions are required",
				}
			}
			return nil
		},
		// examples-required-when-flagged-lowlevel: Low level commands with flags require at least one example
		func(theCmd *cobra.Command, cmdStirng string) *issue.Issue {
			if len(theCmd.Commands()) == 0 && /*No sub commands*/
				theCmd.Flags().HasFlags() && /*At least one flag*/
				len(utilstrings.SplitMultiline(theCmd.Example)) <= 0 /*No example lines*/ {
				return &issue.Issue{
					Target: "cmd: " + cmdStirng,
					Level:  issue.WarningLevel,
					Code:   "examples-required-when-flagged-lowlevel",
					Text:   "Low level commands with flags require at least one example",
				}
			}
			return nil
		},
		// examples-expected: All examples need to start with something that is expected
		// This can be 1) The command name 2) a comment "#" 3) be a blank line (separation)
		// It is common for people to start with `mw`, an alias or for whitespace between lines to be incorrect
		func(theCmd *cobra.Command, cmdStirng string) *issue.Issue {
			for _, line := range utilstrings.SplitMultiline(theCmd.Example) {
				// TODO check all example lines and return an issue for each?
				if len(line) > 0 && strings.Index(line, theCmd.Name()) != 0 && strings.Index(line, "#") != 0 {
					return &issue.Issue{
						Target:  "cmd: " + cmdStirng,
						Level:   issue.ErrorLevel,
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
		func(theCmd *cobra.Command, cmdStirng string) *issue.Issue {
			allowedKeys := []string{
				"exitCode",
				"group",
				"mwcli-lint-skip",
				"mwcli-lint-skip-children",
			}
			// TODO check all annotaitons and issue for each
			for key := range theCmd.Annotations {
				if !utilstrings.StringInSlice(key, allowedKeys) {
					return &issue.Issue{
						Target:  "cmd: " + cmdStirng,
						Level:   issue.ErrorLevel,
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

func detectCobraCommandIssues(theCmd *cobra.Command, cmdString string) []issue.Issue {
	issues := []issue.Issue{}
	for _, detector := range cobraCommandDetectorList() {
		issue := detector(theCmd, cmdString)
		if issue != nil {
			issues = append(issues, *issue)
		}
	}
	return issues
}

func detectCobraCommandIssuesRecursively(theCmd *cobra.Command, parentNamePrefix string) []issue.Issue {
	issues := []issue.Issue{}
	theCmdName := parentNamePrefix + theCmd.Name()

	if len(theCmd.Annotations["mwcli-lint-skip"]) == 0 {
		issues = append(issues, detectCobraCommandIssues(theCmd, theCmdName)...)
	}

	if len(theCmd.Annotations["mwcli-lint-skip-children"]) == 0 {
		for _, nextCmd := range theCmd.Commands() {
			issues = append(issues, detectCobraCommandIssuesRecursively(nextCmd, theCmdName+" ")...)
		}
	}

	return issues
}

func DetectCobraCommandIssuesRoot(rootCmd *cobra.Command) []issue.Issue {
	return detectCobraCommandIssuesRecursively(rootCmd, "")
}
