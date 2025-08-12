package cli

import (
	bytes "bytes"
	encodingjson "encoding/json"
	nethttp "net/http"
	os "os"
	strings "strings"
	time "time"

	"github.com/sirupsen/logrus"
	"gitlab.wikimedia.org/repos/releng/cli/internal/util/files"
)

type Events struct {
	EventFile string
}

func NewEvents(eventFile string) *Events {
	return &Events{EventFile: eventFile}
}

func (e *Events) currentDtString() string {
	currentTime := time.Now()
	return currentTime.Format("2006-01-02T15:04:05.000Z")
}

func (e *Events) AddCommandRunEvent(command string, version Version) {
	e.AddEventToStore(map[string]interface{}{
		"$schema": "/analytics/mwcli/command_execute/1.0.0",
		"meta": map[string]interface{}{
			"stream": "mwcli.command_execute",
		},
		"dt":      e.currentDtString(),
		"command": command,
		"version": version,
	})
}

func (e *Events) AddFeatureUsageEvent(feature string, version Version) {
	e.AddEventToStore(map[string]interface{}{
		"$schema": "/analytics/mwcli/feature_usage/1.0.0",
		"meta": map[string]interface{}{
			"stream": "mwcli.feature_usage",
		},
		"dt":      e.currentDtString(),
		"feature": feature,
		"version": version,
	})
}

func (e *Events) AddEventToStore(event map[string]interface{}) {
	j, _ := encodingjson.Marshal(event)
	files.AddLine(string(j), e.EventFile)
}

func (e *Events) RawEvents() []string {
	return files.Lines(e.EventFile)
}

func (e *Events) EmitEvents() bool {
	eventJSON := e.RawEvents()
	eventCount := len(eventJSON)
	if eventCount == 0 {
		logrus.Debug("No events to emit")
		return false
	}
	logrus.Tracef("Submitting %d events", eventCount)

	payload := []byte("[" + strings.Join(eventJSON, ",") + "]")
	resp, err := nethttp.Post("https://intake-analytics.wikimedia.org/v1/events?hasty=true", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		logrus.Debug(err)
		return false
	}
	defer resp.Body.Close()

	truncateErr := os.Truncate(e.EventFile, 0)
	if truncateErr != nil {
		logrus.Debug(truncateErr)
		return false
	}
	logrus.Debug("Event submission success")
	return true
}
