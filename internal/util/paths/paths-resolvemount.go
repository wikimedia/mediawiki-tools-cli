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
	"strings"
)

/*ResolveMountForCwd ...*/
func ResolveMountForCwd(mountFrom string, mountTo string) *string {
	cwd, _ := os.Getwd()
	return resolveMountForDirectory(mountFrom, mountTo, cwd)
}

func resolveMountForDirectory(mountFrom string, mountTo string, directory string) *string {
	// If the directory that we are in is part of the mount point
	if strings.HasPrefix(directory, mountFrom) {
		// We can use that mount point with any path suffix (other directories) appended
		modified := strings.Replace(directory, mountFrom, mountTo, 1)
		return &modified
	}

	// Otherwise we don't know where we are and can't help
	return nil
}
