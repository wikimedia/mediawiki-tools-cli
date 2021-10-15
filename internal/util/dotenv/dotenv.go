/*Package dotenv for interacting with a .env file

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
package dotenv

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

/*File location of the .env file to work on.*/
type File string

/*FileForDirectory returns the File for the given directory.*/
func FileForDirectory(directory string) File {
	return File(directory + string(os.PathSeparator) + ".env")
}

/*Path the path of the .env file.*/
func (f File) Path() string {
	return string(f)
}

/*EnsureExists ensures that the File exists, creating an empty one if not.*/
func (f File) EnsureExists() {
	if _, err := os.Stat(f.Path()); err != nil {
		err := os.MkdirAll(strings.Replace(f.Path(), filepath.Base(f.Path()), "", -1), 0o700)
		if err != nil {
			panic(err)
		}
		_, err = os.OpenFile(f.Path(), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			panic(err)
		}
	}
}

func (f File) read() map[string]string {
	f.EnsureExists()
	envMap, _ := godotenv.Read(f.Path())
	return envMap
}

func (f File) write(envMap map[string]string) {
	// Override the regular gotdotenv Write method to avoid adding quotes
	// https://github.com/joho/godotenv/issues/50#issuecomment-364873528
	// https://github.com/moby/moby/issues/12997
	// godotenv.Write(envMap, f.Path())
	writeOverride(envMap, f.Path())
}

/*Delete a value from the env.*/
func (f File) Delete(name string) {
	envMap := f.read()
	delete(envMap, name)
	f.write(envMap)
}

/*Set a value in the env.*/
func (f File) Set(name string, value string) {
	envMap := f.read()
	envMap[name] = value
	f.write(envMap)
}

/*Get a value from the env.*/
func (f File) Get(name string) string {
	envMap := f.read()
	return envMap[name]
}

/*Has a value in the env.*/
func (f File) Has(name string) bool {
	envMap := f.read()
	_, ok := envMap[name]
	return ok
}

/*Missing a value in the env.*/
func (f File) Missing(name string) bool {
	return !f.Has(name)
}

/*List all values from the env.*/
func (f File) List() map[string]string {
	return f.read()
}
