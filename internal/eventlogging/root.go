/*Package eventlogging is used for executing commands

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
package eventlogging

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"os/user"
	"strings"
	"time"

	"gitlab.wikimedia.org/releng/cli/internal/util/files"
)

func currentDtString() string {
	currentTime := time.Now()
	return currentTime.Format("2006-01-02T15:04:05.000Z")
}

func AddCommandRunEvent(command string, version string) {
	AddEventToStore(map[string]interface{}{
		"$schema": "/analytics/mwcli/command_execution/1.0.0",
		"meta": map[string]interface{}{
			"stream": "mwcli.command_execution",
		},
		"dt":      currentDtString(),
		"command": command,
		"version": version,
	})
}

func AddEventToStore(event map[string]interface{}) {
	j, _ := json.Marshal(event)
	files.AddLine(string(j), eventFile())
}

func mwcliDirectory() string {
	currentUser, _ := user.Current()
	projectDirectory := currentUser.HomeDir + string(os.PathSeparator) + ".mwcli"
	return projectDirectory
}

func eventFile() string {
	return mwcliDirectory() + "/.events"
}

func EmitEvents() bool {
	eventJsons := files.Lines(eventFile())
	if len(eventJsons) == 0 {
		return false
	}

	payload := []byte("[" + strings.Join(eventJsons, ",") + "]")
	_, err := http.Post("https://intake-analytics.wikimedia.org/v1/events?hasty=true", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		// TODO log in verbose
		// log.Fatal(err)
		return false
	}

	files.RemoveIfExists(eventFile())
	return true
}
