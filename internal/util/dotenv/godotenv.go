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
	"fmt"
	"os"
	"sort"
	"strings"
)

// Copy of godotenv.Write with modifications to never quote
// https://github.com/joho/godotenv/issues/50#issuecomment-364873528
// https://github.com/moby/moby/issues/12997
func writeOverride(envMap map[string]string, filename string) error {
	content, error := marshelOverride(envMap)
	if error != nil {
		return error
	}
	file, error := os.Create(filename)
	if error != nil {
		return error
	}
	_, err := file.WriteString(content)
	return err
}

// Copy of godotenv.Marshel with modifications to never quote
// https://github.com/joho/godotenv/issues/50#issuecomment-364873528
// https://github.com/moby/moby/issues/12997
func marshelOverride(envMap map[string]string) (string, error) {
	lines := make([]string, 0, len(envMap))
	for k, v := range envMap {
		lines = append(lines, fmt.Sprintf(`%s=%s`, k, doubleQuoteEscape(v)))
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n"), nil
}

const doubleQuoteSpecialChars = "\\\n\r\"!$`"

func doubleQuoteEscape(line string) string {
	for _, c := range doubleQuoteSpecialChars {
		toReplace := "\\" + string(c)
		if c == '\n' {
			toReplace = `\n`
		}
		if c == '\r' {
			toReplace = `\r`
		}
		line = strings.Replace(line, string(c), toReplace, -1)
	}
	return line
}
