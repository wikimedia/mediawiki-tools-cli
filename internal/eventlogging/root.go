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
		"$schema": "/analytics/mwcli/command_execute/1.0.0",
		"meta": map[string]interface{}{
			"stream": "mwcli.command_execute",
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
