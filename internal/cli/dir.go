package cli

import (
	"os"

	"gitlab.wikimedia.org/releng/cli/internal/util/dirs"
)

/*MWCLIDIR ... */
const MWCLIDIR string = ".mwcli"

/*UserDirectoryPathForCmd is a path within the .mwcli directory of the user directory that can be used by a command for storage*/
func UserDirectoryPathForCmd(cmdName string) string {
	return dirs.UserDirectoryPath(MWCLIDIR + string(os.PathSeparator) + cmdName)
}
