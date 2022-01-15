package cobra

import (
	"strings"

	"github.com/spf13/cobra"
)

/*FullCommandString for example "docker redis exec"*/
func FullCommandString(cmd *cobra.Command) string {
	s := ""
	for cmd.HasParent() {
		s = cmd.Name() + " " + s
		cmd = cmd.Parent()
	}
	return strings.Trim(s, " ")
}
