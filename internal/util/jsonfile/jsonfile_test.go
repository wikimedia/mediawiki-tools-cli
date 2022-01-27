/*Package jsonfile for interacting with a single json file on disk

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
package jsonfile

import (
	"testing"
)

func TestJSONFile_Clear(t *testing.T) {
	t.Run("clear clears json content", func(t *testing.T) {
		jEmpty := LoadFromDisk("./test_empty.json")
		jNonEmpty := LoadFromDisk("./test_nonEmpty.json")
		jNonEmpty.Clear()
		if jEmpty.String() != jNonEmpty.String() {
			t.Error("Expected json to be empty")
		}
	})
}

func TestJSONFile_String(t *testing.T) {
	tests := []struct {
		name     string
		loadFile string
		want     string
	}{
		{
			name:     "empty looks empty",
			loadFile: "./test_empty.json",
			want:     "{}",
		},
		{
			name:     "content is printed",
			loadFile: "./test_nonEmpty.json",
			want: `{
  "foo": "bar"
}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := LoadFromDisk(tt.loadFile)
			if got := j.String(); got != tt.want {
				t.Errorf("JSONFile.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
