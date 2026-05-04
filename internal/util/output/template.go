package output

import (
	"bytes"
	"encoding/json"
	"io"
	"text/template"

	"github.com/sirupsen/logrus"
)

// printTemplate executes a Go text/template against objects and writes the result to writer.
// If format is empty it falls back to plain JSON output.
func printTemplate(objects interface{}, format string, writer io.Writer) {
	if format == "" {
		NewJSON(objects, "").Print(writer)
		return
	}

	// Normalise via JSON round-trip so template can access fields uniformly
	var data interface{}
	b, err := json.Marshal(objects)
	if err != nil {
		logrus.Errorf("template: marshal error: %v", err)
		return
	}
	if err := json.Unmarshal(b, &data); err != nil {
		logrus.Errorf("template: unmarshal error: %v", err)
		return
	}

	tmpl, err := template.New("output").Parse(format)
	if err != nil {
		logrus.Errorf("template: parse error: %v", err)
		return
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		logrus.Errorf("template: execute error: %v", err)
		return
	}
	if _, err := io.Copy(writer, &buf); err != nil {
		logrus.Errorf("template: write error: %v", err)
	}
}
