/*Package timers in internal utils is functionality for interacting with timers stored in config

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
package timers

import (
	"reflect"
	"testing"
	"time"
)

func TestNowUTC(t *testing.T) {
	clockOverride = "2020-01-01T10:00:00Z"

	tests := []struct {
		name string
		want time.Time
	}{
		{
			name: "returns the current time in UTC",
			want: Parse(clockOverride),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NowUTC(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NowUTC() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHoursAgo(t *testing.T) {
	clockOverride = "2020-01-01T10:00:00Z"
	twoHoursBefore, _ := time.Parse(time.RFC3339, "2020-01-01T08:00:00Z")

	type args struct {
		hours int
	}
	tests := []struct {
		name string
		args args
		want time.Time
	}{
		{
			name: "2 hours before",
			args: args{
				hours: 2,
			},
			want: twoHoursBefore,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HoursAgo(tt.args.hours); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("HoursAgo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsHoursAgo(t *testing.T) {
	clockOverride = "2020-01-01T10:00:00Z"
	twoHoursBefore, _ := time.Parse(time.RFC3339, "2020-01-01T08:00:00Z")

	type args struct {
		t     time.Time
		hours float64
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "2 hours before is over 3 hours ago",
			args: args{
				t:     twoHoursBefore,
				hours: 3,
			},
			want: false,
		},
		{
			name: "2 hours before is exactly 2 hours ago",
			args: args{
				t:     twoHoursBefore,
				hours: 2,
			},
			want: true,
		},
		{
			name: "2 hours before is NOT 1 hour ago",
			args: args{
				t:     twoHoursBefore,
				hours: 1,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsHoursAgo(tt.args.t, tt.args.hours); got != tt.want {
				t.Errorf("IsHoursAgo() = %v, want %v", got, tt.want)
			}
		})
	}
}
