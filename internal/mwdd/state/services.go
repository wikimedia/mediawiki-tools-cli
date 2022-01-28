/*Records the state of the various services that have been created or stopped.
This enables various commands , such as stop and start, to be more selective
about which services they specify.

For example, if you run `create mediawiki` and then `suspend` and `resume`
you do NOT want to see errors about starting servcices that you didn't create
in the first place.

Copyright © 2022 Addshore

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
package state

import (
	"os"

	"gitlab.wikimedia.org/releng/cli/internal/mwdd"
	"gitlab.wikimedia.org/releng/cli/internal/util/jsonfile"
)

func RecordServiceCreated(service string) {
	j := jsonFileForStorage()
	j.Contents[service] = true
	j.WriteToDisk()
}

func RecordServicesCreated(services []string) {
	j := jsonFileForStorage()
	for _, service := range services {
		j.Contents[service] = true
	}
	j.WriteToDisk()
}

func RecordServiceDestroyed(service string) {
	j := jsonFileForStorage()
	delete(j.Contents, service)
	j.WriteToDisk()
}

func RecordServicesDestroyed(services []string) {
	j := jsonFileForStorage()
	for _, service := range services {
		delete(j.Contents, service)
	}
	j.WriteToDisk()
}

func ClearServiceState() {
	j := jsonFileForStorage()
	j.Clear()
	j.WriteToDisk()
}

func jsonFileForStorage() jsonfile.JSONFile {
	return jsonfile.LoadFromDisk(jsonPathForStorage())
}

func jsonPathForStorage() string {
	return mwdd.DefaultForUser().Directory() + string(os.PathSeparator) + "state-servicesets.json"
}
