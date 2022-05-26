package cobra

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

// FullCommandStringWithoutPrefix removes an optional prefix from FullCommandString
// This can be used to, for example, remove the root command from the string with ease.
func FullCommandStringWithoutPrefix(cmd *cobra.Command, prefix string) string {
	return strings.Trim(strings.TrimPrefix(FullCommandString(cmd), prefix), " ")
}

// CommandIsSubCommandOf detects if a command is a subcommand (or command) or a given command string
// Example: CommandIsSubCommandOf(cmd, "mw docker env") returns true if cmd is a subcommand of docker env command
// Canonical names must be used, aliases will not work.
func CommandIsSubCommandOf(cmd *cobra.Command, subCommandString string) bool {
	fullCommandString := FullCommandString(cmd)
	return strings.HasPrefix(fullCommandString, subCommandString)
}

func CommandIsSubCommandOfOneOrMore(cmd *cobra.Command, subCommandStrings []string) bool {
	for _, subCommandString := range subCommandStrings {
		if CommandIsSubCommandOf(cmd, subCommandString) {
			return true
		}
	}
	return false
}

func VisitAllCommands(cmd *cobra.Command, fn func(*cobra.Command)) {
	fn(cmd)
	for _, subCmd := range cmd.Commands() {
		VisitAllCommands(subCmd, fn)
	}
}
