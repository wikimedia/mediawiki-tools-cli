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

import "testing"

func TestFilterMultiline(t *testing.T) {
	type args struct {
		s               string
		requiredMatches []string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "no match",
			args: args{
				s:               "foo",
				requiredMatches: []string{"bar"},
			},
			want: "",
		},
		{
			name: "match single",
			args: args{
				s:               "foo\nbar",
				requiredMatches: []string{"bar"},
			},
			want: "bar",
		},
		{
			name: "match multiple lines",
			args: args{
				s:               "foo\nbar\nbaz",
				requiredMatches: []string{"ba"},
			},
			want: "bar\nbaz",
		},
		{
			name: "match multiple search",
			args: args{
				s:               "foo\nbar\nbaz",
				requiredMatches: []string{"b", "a"},
			},
			want: "bar\nbaz",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FilterMultiline(tt.args.s, tt.args.requiredMatches); got != tt.want {
				t.Errorf("FilterMultiline() = %v, want %v", got, tt.want)
			}
		})
	}
}
