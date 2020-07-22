/*Package env for interacting with the .env file of the environment

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
	"os"

	"github.com/joho/godotenv"
)

func ensureDotEnvFile() {
	if _, err := os.Stat(GetPath()); err != nil {
		os.OpenFile(GetPath(), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	}
}

/*GetProjectPath for the environment*/
func GetProjectPath() string {
	projectDir, _ := os.Getwd()
	return projectDir
}

/*GetPath for the environment*/
func GetPath() string {
	return GetProjectPath() + string(os.PathSeparator) + ".env"
}

func read() map[string]string {
	ensureDotEnvFile()
	envMap, _ := godotenv.Read(GetPath())
	return envMap
}

func write(envMap map[string]string) {
	godotenv.Write(envMap, GetPath())
}

/*Delete a value from the env*/
func Delete(name string) {
	envMap := read()
	delete(envMap, name)
	write(envMap)
}

/*Set a value in the env*/
func Set(name string, value string) {
	envMap := read()
	envMap[name] = value
	write(envMap)
}

/*Get a value from the env*/
func Get(name string) string {
	envMap := read()
	return envMap[name]
}

/*List all values from the env*/
func List() map[string]string {
	return read()
}
