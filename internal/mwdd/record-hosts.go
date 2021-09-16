/*Package mwdd is used to interact a mwdd v2 setup

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
package mwdd

import (
	"os"

	"gerrit.wikimedia.org/r/mediawiki/tools/cli/internal/util/files"
)

func (m MWDD) hostRecordFile() string {
	return m.Directory() + string(os.PathSeparator) + "record-hosts"
}

/*RecordHostUsageBySite records a host in a local file as used at some point*/
func (m MWDD) RecordHostUsageBySite(host string) {
	files.AddLineUnique(host, m.hostRecordFile())
}

/*UsedHosts lists all hosts that have been used at some point*/
func (m MWDD) UsedHosts() []string {
	return files.Lines(m.hostRecordFile())
}
