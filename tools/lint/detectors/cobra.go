package detectors

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	stringsutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/strings"
	"gitlab.wikimedia.org/repos/releng/cli/tools/lint/issue"
)

func cobraCommandDetectorList() []func(*cobra.Command, string) *issue.Issue {
	return []func(*cobra.Command, string) *issue.Issue{
		detectLongHelp,
		detectLongExamples,
		detectLongHelpRequired,
		detectPositionalArgsValidated,
		detectFlagNames,
		detectFlagHelp,
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
				hasLocalFlags(theCmd) && /*At least one local flag*/
				len(stringsutil.SplitMultiline(theCmd.Example)) <= 0 /*No example lines*/ {
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
		// This can be
		// 1) The command name
		// 2) a comment "#" (for a comment)
		// 3) be a blank line (separation)
		// 4) be two spaces, follow by the command name
		// 5) be two spaces, follow by a "#" (for a comment)
		// It is common for people to start with `mw`, an alias or for whitespace between lines to be incorrect
		func(theCmd *cobra.Command, cmdStirng string) *issue.Issue {
			for _, line := range stringsutil.SplitMultiline(theCmd.Example) {
				// TODO check all example lines and return an issue for each?
				if len(line) > 0 && strings.Index(line, theCmd.Name()) != 0 && strings.Index(line, "#") != 0 && strings.Index(line, "  "+theCmd.Name()) != 0 && strings.Index(line, "  #") != 0 {
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
				"mwcli-docs-skip-children",
			}
			// TODO check all annotaitons and issue for each
			for key := range theCmd.Annotations {
				if !stringsutil.StringInSlice(key, allowedKeys) {
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
		// end-to-end-test: All commands should have an end to end test
		// Helps ensure test coverage
		func(theCmd *cobra.Command, cmdStirng string) *issue.Issue {
			testFound := false
			err := filepath.Walk("tests", func(path string, info os.FileInfo, errAbove error) error {
				if info.IsDir() || strings.Split(info.Name(), "-")[0] != "test" {
					return nil
				}

				file, err := os.Open(filepath.Clean(path))
				if err != nil {
					panic(err)
				}
				defer file.Close()

				scanner := bufio.NewScanner(file)
				for scanner.Scan() {
					if strings.Contains(scanner.Text(), cmdStirng) {
						testFound = true
					}
				}
				return nil
			})
			if err != nil {
				panic(err)
			}
			if !testFound {
				return &issue.Issue{
					Target: "cmd: " + cmdStirng,
					Level:  issue.SuggestLevel,
					Code:   "end-to-end-test",
					Text:   "End to end tests are suggested, none found",
				}
			}
			return nil
		},
	}
}

// detectFlagNames suggests lowercase flag names with hyphen-separated words.
func detectFlagNames(cmd *cobra.Command, path string) *issue.Issue {
	for _, flags := range []*pflag.FlagSet{cmd.LocalNonPersistentFlags(), cmd.PersistentFlags()} {
		var badNames []string
		flags.VisitAll(func(flag *pflag.Flag) {
			if !isKebabFlagName(flag.Name) {
				badNames = append(badNames, "--"+flag.Name)
			}
		})
		if len(badNames) > 0 {
			return &issue.Issue{
				Target: "cmd: " + path, Level: issue.SuggestLevel, Code: "flag-name-style",
				Text:    "Use lowercase, hyphen-separated flag names",
				Context: strings.Join(badNames, ", "),
			}
		}
	}
	return nil
}

func isKebabFlagName(name string) bool {
	if name == "" || name[0] < 'a' || name[0] > 'z' {
		return false
	}
	previousHyphen := false
	for i := 1; i < len(name); i++ {
		char := name[i]
		if char == '-' {
			if previousHyphen || i == len(name)-1 {
				return false
			}
			previousHyphen = true
			continue
		}
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') {
			return false
		}
		previousHyphen = false
	}
	return true
}

// detectFlagHelp warns when a local flag has no description for --help output.
func detectFlagHelp(cmd *cobra.Command, path string) *issue.Issue {
	for _, flags := range []*pflag.FlagSet{cmd.LocalNonPersistentFlags(), cmd.PersistentFlags()} {
		var undocumented []string
		flags.VisitAll(func(flag *pflag.Flag) {
			if strings.TrimSpace(flag.Usage) == "" {
				undocumented = append(undocumented, "--"+flag.Name)
			}
		})
		if len(undocumented) > 0 {
			return &issue.Issue{
				Target: "cmd: " + path, Level: issue.WarningLevel, Code: "flag-help-required",
				Text:    "Add help text for every flag",
				Context: strings.Join(undocumented, ", "),
			}
		}
	}
	return nil
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
		issues = append(issues, detectSiblingCommandNameCollisions(theCmd, theCmdName)...)
		for _, nextCmd := range theCmd.Commands() {
			issues = append(issues, detectCobraCommandIssuesRecursively(nextCmd, theCmdName+" ")...)
		}
	}

	return issues
}

// detectLongHelpRequired suggests long help for executable leaf commands whose
// flags or positional arguments need more explanation than a one-line summary.
func detectLongHelpRequired(cmd *cobra.Command, path string) *issue.Issue {
	if len(cmd.Commands()) != 0 || (cmd.Run == nil && cmd.RunE == nil) || strings.TrimSpace(cmd.Long) != "" {
		return nil
	}
	if !hasLocalFlags(cmd) && !useHasPositionalArgs(cmd.Use) {
		return nil
	}
	return &issue.Issue{
		Target: "cmd: " + path, Level: issue.SuggestLevel, Code: "long-help-required",
		Text: "Add long help to explain arguments, flags, and behavior",
	}
}

func hasLocalFlags(cmd *cobra.Command) bool {
	return cmd.LocalNonPersistentFlags().HasFlags() || cmd.PersistentFlags().HasFlags()
}

// detectPositionalArgsValidated warns when Use documents positional arguments
// but Cobra is not configured to reject missing or unexpected arguments.
func detectPositionalArgsValidated(cmd *cobra.Command, path string) *issue.Issue {
	if (cmd.Run == nil && cmd.RunE == nil) || cmd.DisableFlagParsing || cmd.Args != nil || !useHasPositionalArgs(cmd.Use) {
		return nil
	}
	return &issue.Issue{
		Target: "cmd: " + path, Level: issue.WarningLevel, Code: "positional-args-unvalidated",
		Text: "Add a Cobra Args validator so invalid positional arguments fail with usage",
	}
}

func useHasPositionalArgs(use string) bool {
	fields := strings.Fields(use)
	if len(fields) < 2 {
		return false
	}
	for _, field := range fields[1:] {
		if field == "[flags]" || field == "--" {
			continue
		}
		argument := strings.Trim(field, "[]<>{}")
		if strings.HasPrefix(argument, "-") {
			continue
		}
		return true
	}
	return false
}

// detectSiblingCommandNameCollisions reports names or aliases that shadow a
// sibling command name or alias and make command lookup ambiguous.
func detectSiblingCommandNameCollisions(parent *cobra.Command, parentPath string) []issue.Issue {
	owners := map[string]*cobra.Command{}
	var issues []issue.Issue
	for _, child := range parent.Commands() {
		names := append([]string{child.Name()}, child.Aliases...)
		for _, name := range names {
			if previous, ok := owners[name]; ok && previous != child {
				issues = append(issues, issue.Issue{
					Target: "cmd: " + parentPath + " " + child.Name(), Level: issue.ErrorLevel,
					Code:    "command-name-collision",
					Text:    "Command names and aliases must be unique among siblings",
					Context: name + " is also used by " + parentPath + " " + previous.Name(),
				})
				continue
			}
			owners[name] = child
		}
	}
	return issues
}

func DetectCobraCommandIssuesRoot(rootCmd *cobra.Command) []issue.Issue {
	return detectCobraCommandIssuesRecursively(rootCmd, "")
}
