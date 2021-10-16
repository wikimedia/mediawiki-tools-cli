/*Package files in internal utils is functionality for interacting with files in generic ways

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
	"bufio"
	"bytes"
	"io/ioutil"
	"os"
	"strings"
)

/*AddLinesUnique adds all lines to the file if each one will be the only occourance of the string.*/
func AddLinesUnique(lines []string, filename string) {
	for _, line := range lines {
		AddLineUnique(line, filename)
	}
}

/*AddLineUnique adds the line to the file if it will be the only occourance of the string.*/
func AddLineUnique(line string, fileName string) {
	file, err := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	buf := new(bytes.Buffer)
	buf.ReadFrom(file)
	s := buf.String()

	if !strings.Contains(s, line) {
		if _, err := file.WriteString(line + "\n"); err != nil {
			panic(err)
		}
	}
}

/*Lines reads all lines from a file.*/
func Lines(fileName string) []string {
	_, err := os.Stat(fileName)
	if err != nil {
		return []string{}
	}
	file, err := os.Open(fileName)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}

/*Bytes gets bytes of a file or panics.*/
func Bytes(fileName string) []byte {
	bytes, err := ioutil.ReadFile(fileName)
	if err != nil {
		panic(err)
	}
	return bytes
}
