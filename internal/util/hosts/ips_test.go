/*Package hosts in internal utils is functionality for interacting with an etc hosts file

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
package hosts

import (
	"testing"
)

func TestLocalhostIps(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "localhost has 127.0.0.1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := IPs("localhost")

			if !splitContains(got, "127.0.0.1") {
				t.Errorf("LocalhostIps() = doesn't container 127.0.0.1 and probably should, got: %v", got)
				return
			}
		})
	}
}

func splitContains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
