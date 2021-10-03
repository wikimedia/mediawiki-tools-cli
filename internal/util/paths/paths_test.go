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
	"os/user"
	"testing"
)

func TestFullifyUserProvidedPath(t *testing.T) {
	usr, _ := user.Current()
	usrDir := usr.HomeDir

	tests := []struct {
		name  string
		given string
		want  string
	}{
		{
			name:  "Passthrough",
			given: "/foo",
			want:  "/foo",
		},
		{
			name:  "User dir 1",
			given: "~",
			want:  usrDir,
		},
		{
			name:  "User dir 2",
			given: "~/",
			want:  usrDir,
		},
		{
			name:  "User sub dir",
			given: "~/foo",
			want:  usrDir + "/foo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FullifyUserProvidedPath(tt.given); got != tt.want {
				t.Errorf("FullifyUserProvidedPath() = %v, want %v", got, tt.want)
			}
		})
	}
}
