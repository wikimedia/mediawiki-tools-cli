package cobrautil

import (
	"strings"

	"github.com/spf13/cobra"
)

// FullCommandString for example "mw docker redis exec".
func FullCommandString(cmd *cobra.Command) string {
	s := cmd.Name()
	for cmd.HasParent() {
		s = cmd.Parent().Name() + " " + s
		cmd = cmd.Parent()
	}
	return strings.Trim(s, " ")
}

// FullCommandStrings returns a list of all full command strings for a given command.
// This includes variations with aliases at all levels.
func FullCommandStrings(cmd *cobra.Command) []string {
	strings := []string{}
	strings = append(strings, cmd.Name())
	for _, alias := range cmd.Aliases {
		strings = append(strings, alias)
	}

	for cmd.HasParent() {
		thisLevelStrings := []string{}
		thisLevelStrings = append(thisLevelStrings, cmd.Parent().Name())
		for _, alias := range cmd.Parent().Aliases {
			thisLevelStrings = append(thisLevelStrings, alias)
		}

		// For each this level string, multiply out the strings from the next level, so we have all combinations
		newStrings := []string{}
		for _, thisLevelString := range thisLevelStrings {
			for _, s := range strings {
				newStrings = append(newStrings, thisLevelString+" "+s)
			}
		}
		strings = newStrings

		cmd = cmd.Parent()
	}

	return strings
}

// FullCommandStringWithoutPrefix removes an optional prefix from FullCommandString
// This can be used to, for example, remove the root command from the string with ease.
func FullCommandStringWithoutPrefix(cmd *cobra.Command, prefix string) string {
	return strings.Trim(strings.TrimPrefix(FullCommandString(cmd), prefix), " ")
}

// CommandIsSubCommandOfString detects if a command is a subcommand of a given command string.
// Example: CommandIsSubCommandOfString(cmd, "mw docker env") returns true if cmd is a subcommand of docker env command.
func CommandIsSubCommandOfString(cmd *cobra.Command, cmdString string) bool {
	fullCommandStrings := FullCommandStrings(cmd)

	for _, fullCommandString := range fullCommandStrings {
		if strings.HasPrefix(fullCommandString, cmdString) {
			return true
		}
	}

	return false
}

// Same as CommandIsSubCommandOfString but for multiple strings
func CommandIsSubCommandOfOneOrMoreStrings(cmd *cobra.Command, subCommandStrings []string) bool {
	for _, subCommandString := range subCommandStrings {
		if CommandIsSubCommandOfString(cmd, subCommandString) {
			return true
		}
	}
	return false
}

// VisitAllCommands visits all commands and subcommands of a given command
// It can be used to perform alterations on a tree of commands that perhaps you havn't created yourself.
func VisitAllSubCommands(cmd *cobra.Command, fn func(*cobra.Command)) {
	fn(cmd)
	for _, subCmd := range cmd.Commands() {
		VisitAllSubCommands(subCmd, fn)
	}
}

// AllFullCommandStringsFromParent returns all full command strings for all subcommands of a given command
// This includes variations with aliases at all levels
// This can be used to check if a command is a subcommand of a given cmd
func AllFullCommandStringsFromParent(cmd *cobra.Command) []string {
	strings := []string{}
	VisitAllSubCommands(cmd, func(c *cobra.Command) {
		strings = append(strings, FullCommandStrings(c)...)
	})
	return strings
}
