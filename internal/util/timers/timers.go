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
	"log"
	"time"
)

var clockOverride = ""

func NowUTC() time.Time {
	if clockOverride != "" {
		return Parse(clockOverride)
	}
	return time.Now().UTC()
}

func HoursAgo(hours int) time.Time {
	return NowUTC().Add(-time.Duration(hours) * time.Hour)
}

func IsHoursAgo(t time.Time, hours float64) bool {
	return NowUTC().Sub(t).Hours() >= hours
}

func String(t time.Time) string {
	return t.Format(time.RFC3339)
}

func Parse(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		log.Fatal(err)
	}
	return t
}
