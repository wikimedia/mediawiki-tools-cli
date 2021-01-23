/*Package files for interacting packaged files and their counterparts on disk for a project directory

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
package files

import (
	"os"
	"path/filepath"
)

/*EnsureReady makes sure that the files component is ready*/
func EnsureReady( projectDirectory string ) {
	ensureInMemoryFilesAreOnDisk( projectDirectory );
}

/*ListRawDcYamlFilesInContextOfProjectDirectory ...*/
func ListRawDcYamlFilesInContextOfProjectDirectory(projectDirectory string) []string {
	// TODO this function should live in the mwdd struct?
	var files []string

	for _, file := range listRawFiles(projectDirectory) {
		if( filepath.Ext(file) == ".yml" ) {
			files = append(files, filepath.Base(file))
		}
	}

	return files
}

/*listRawFiles lists the raw docker-compose file paths that are currently on disk*/
func listRawFiles(projectDirectory string) []string {
	var files []string

	err := filepath.Walk(projectDirectory, func(path string, info os.FileInfo, err error) error {
		if(! info.IsDir()){
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	return files
}