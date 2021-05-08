/*Package env for interacting with a .env file

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
package env

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

/*DotFile representation of a .env file*/
type DotFile string

/*DotFileForDirectory returns a dotFIle for the given directory*/
func DotFileForDirectory(directory string) DotFile {
	return DotFile(directory + string(os.PathSeparator) + ".env")
}

/*Path the path of the .env file*/
func (f DotFile) Path() string {
	return string(f)
}

/*EnsureExists ensures that the .env file exists, creating an empty one if not*/
func (f DotFile) EnsureExists() {
	if _, err := os.Stat(f.Path()); err != nil {
		err := os.MkdirAll(strings.Replace(f.Path(), ".env", "", -1), 0700)
		if err != nil {
			log.Fatal(err)
		}
		_, err = os.OpenFile(f.Path(), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func (f DotFile) read() map[string]string {
	f.EnsureExists()
	envMap, _ := godotenv.Read(f.Path())
	return envMap
}

func (f DotFile) write(envMap map[string]string) {
	godotenv.Write(envMap,f.Path())
}

/*Delete a value from the env*/
func (f DotFile) Delete(name string) {
	envMap := f.read()
	delete(envMap, name)
	f.write(envMap)
}

/*Set a value in the env*/
func (f DotFile) Set(name string, value string) {
	envMap := f.read()
	envMap[name] = value
	f.write(envMap)
}

/*Get a value from the env*/
func (f DotFile) Get(name string) string {
	envMap := f.read()
	return envMap[name]
}

/*Has a value in the env*/
func (f DotFile) Has(name string) bool {
	envMap := f.read()
	_, ok := envMap[name]
	return ok
}

/*Missing a value in the env*/
func (f DotFile) Missing(name string) bool {
	return !f.Has(name)
}

/*List all values from the env*/
func (f DotFile) List() map[string]string {
	return f.read()
}