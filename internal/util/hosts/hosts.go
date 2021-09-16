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
	"io/ioutil"
	"os"
	"strings"

	"github.com/txn2/txeh"
)

var hostsFile = ""
var hostsTmpPrefix = "mwcli-hosts-"

/*Save result of saving the hosts file*/
type Save struct {
	Success bool
	Content string
	TmpFile string
}

/*AddHosts attempts to add requested hosts to the system hosts file, and gives you the new content, a tmp file and success*/
func AddHosts(toAdd []string) Save {
	hosts := hosts()
	hosts.AddHosts("127.0.0.1", toAdd)
	return save(hosts)
}

/*RemoveHostsWithSuffix attempts to remove all hosts with a suffix from the system hosts file, and gives you the new content, a tmp file and success*/
func RemoveHostsWithSuffix(hostSuffix string) Save {
	hosts := hosts()
	removeHostsWithSuffixFromLines(hostSuffix, hosts)
	return save(hosts)
}

func tmpFile() string {
	tmpFile, err := ioutil.TempFile(os.TempDir(), hostsTmpPrefix)
	if err != nil {
		panic(err)
	}
	tmpFile.Close()
	return tmpFile.Name()
}

func hosts() *txeh.Hosts {
	hosts, err := txeh.NewHosts(&txeh.HostsConfig{
		ReadFilePath:  hostsFile,
		WriteFilePath: hostsFile,
	})
	if err != nil {
		panic(err)
	}
	return hosts
}

func removeHostsWithSuffixFromLines(hostSuffix string, hosts *txeh.Hosts) *txeh.Hosts {
	lines := hosts.GetHostFileLines()

	for _, line := range *lines {
		for _, hostname := range line.Hostnames {
			if strings.HasSuffix(hostname, hostSuffix) {
				hosts.RemoveHost(hostname)
			}
		}
	}

	return hosts
}

func save(hosts *txeh.Hosts) Save {
	err := hosts.Save()
	if err != nil {
		tmpFile := tmpFile()
		err = hosts.SaveAs(tmpFile)
		if err != nil {
			panic(err)
		}
		return Save{
			Success: false,
			Content: hosts.RenderHostsFile(),
			TmpFile: tmpFile,
		}
	}

	return Save{
		Success: true,
		Content: hosts.RenderHostsFile(),
		TmpFile: hosts.WriteFilePath,
	}
}
