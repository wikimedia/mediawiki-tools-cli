package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"text/template"

	"github.com/sirupsen/logrus"
)

// printTemplate executes a Go text/template against objects and writes the result to writer.
// If format is empty it falls back to plain JSON output.
//
//nolint:unused
func printTemplate(objects interface{}, format string, writer io.Writer) {
	if format == "" {
		NewJSON(objects, "").Print(writer)
		return
	}

	// Normalise via JSON round-trip so template can access fields uniformly
	var data interface{}
	b, err := json.Marshal(normalizeForTemplate(objects))
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

// normalizeForTemplate recursively converts maps with non-string keys into
// map[string]interface{} so they can be marshaled to JSON.
//
//nolint:unused
func normalizeForTemplate(in interface{}) interface{} {
	switch typed := in.(type) {
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(typed))
		for key, value := range typed {
			out[fmt.Sprintf("%v", key)] = normalizeForTemplate(value)
		}
		return out
	case map[string]interface{}:
		out := make(map[string]interface{}, len(typed))
		for key, value := range typed {
			out[key] = normalizeForTemplate(value)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(typed))
		for i, value := range typed {
			out[i] = normalizeForTemplate(value)
		}
		return out
	default:
		return in
	}
}
