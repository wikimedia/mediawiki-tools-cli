/*Package paths in internal utils is functionality for interacting with paths in generic ways

Copyright © 2020 Addshore

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/
package paths

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

/*FullifyUserProvidedPath fullify people entering ~/ or ./ paths and them not being handeled anywhere.*/
func FullifyUserProvidedPath(userProvidedPath string) string {
	currentWorkingDirectory, _ := os.Getwd()

	if userProvidedPath == "." {
		return currentWorkingDirectory
	}
	if strings.HasPrefix(userProvidedPath, "./") {
		return filepath.Join(currentWorkingDirectory, userProvidedPath[2:])
	}

	usr, _ := user.Current()
	usrDir := usr.HomeDir

	if userProvidedPath == "~" {
		return usrDir
	}
	if strings.HasPrefix(userProvidedPath, "~/") {
		return filepath.Join(usrDir, userProvidedPath[2:])
	}

	// Fallback to what we were provided
	return userProvidedPath
}
