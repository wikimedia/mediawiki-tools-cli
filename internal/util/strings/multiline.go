/*Package strings in internal utils is functionality for interacting with strings

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
package strings

import (
	"bufio"
	"strings"
)

/*FilterMultiline ...*/
func FilterMultiline(s string, requiredMatches []string) string {
	scanner := bufio.NewScanner(strings.NewReader(s))
	out := ""
	for scanner.Scan() {
		okay := true
		for _, arg := range requiredMatches {
			if !strings.Contains(scanner.Text(), arg) {
				okay = false
			}
		}
		if okay {
			out = out + scanner.Text() + "\n"
		}
	}
	return strings.Trim(out, "\n")
}

/*SplitMultiline ...*/
func SplitMultiline(s string) []string {
	scanner := bufio.NewScanner(strings.NewReader(s))
	out := []string{}
	for scanner.Scan() {
		out = append(out, scanner.Text())
	}
	return out
}
